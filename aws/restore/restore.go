package restore

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/herdius/herdius-core/blockchain"
	"github.com/herdius/herdius-core/blockchain/protobuf"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/herdius/herdius-core/config"
)

type RestorerI interface {
	Restore() error
	testCompleteChainRemote() (bool, error)
	clearOld() error
	downloadChain() (*[]protobuf.BaseBlock, error)
	replayChain(*[]protobuf.BaseBlock) error
}

type Restorer struct {
	statePath       string
	chainPath       string
	s3bucket        string
	heightToRestore int
	s3              *s3.S3
}

func NewRestorer(env string, height int) RestorerI {
	detail := config.GetConfiguration(env)
	s := s3.New(session.New())
	return Restorer{
		statePath:       detail.StateDBPath,
		chainPath:       detail.ChainDBPath,
		s3bucket:        detail.S3Bucket,
		heightToRestore: height,
		s3:              s,
	}
}

// Restore retrieves and procceses an entire blockchain stored in S3
// into the Supervisor's local blockchain and statedb
func (r Restorer) Restore() error {
	succ, err := r.testCompleteChainRemote()
	if err != nil {
		err = fmt.Errorf("restore failed while trying to test remote chain: %v", err)
	}
	if !succ {
		err = fmt.Errorf("could not restore chain from backup, specified chain in S3 is invalid")
	}

	err = r.clearOld()
	if err != nil {
		return fmt.Errorf("restore failed while trying to clean old chain: %v", err)
	}

	err = r.downloadState()
	if err != nil {
		return fmt.Errorf("restore failed while trying to download state db: %v", err)
	}

	jsonBlocks, err := r.downloadChain()
	if err != nil {
		return fmt.Errorf("restore failed while trying to download backed up chain: %v", err)
	}

	err = r.replayChain(jsonBlocks)
	if err != nil {
		return fmt.Errorf("restore failed while trying to replay chain: %v", err)
	}

	return nil
}

func (r Restorer) testCompleteChainRemote() (bool, error) {
	listParams := &s3.ListObjectsV2Input{
		Bucket:  aws.String(r.s3bucket),
		Prefix:  aws.String("0/blocks/"),
		MaxKeys: aws.Int64(1),
	}
	listResult, err := r.s3.ListObjectsV2(listParams)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve list of S3 objects: %v", err)
	}
	if len(listResult.Contents) != 1 {
		return false, fmt.Errorf("failed to find base block in S3 (block height = 0)")
	}
	log.Printf("root base block: %+v", *listResult.Contents[0].Key)
	key := *listResult.Contents[0].Key
	downloadParams := &s3.GetObjectInput{
		Bucket: aws.String(r.s3bucket),
		Key:    aws.String(key),
	}

	for i := 0; i < r.heightToRestore; i++ {
		downloadParams.Key = aws.String(key)
		downResult, err := r.s3.GetObject(downloadParams)
		if err != nil {
			return false, fmt.Errorf("failed to download S3 objects (height=%v, key=%v): %v", i, key, err)
		}

		key, err = r.getKeyFromDownload(i+1, downResult)
		if err != nil {
			return false, fmt.Errorf("failed to get key from prior block download (height=%v): %v", i, err)
		}

	}
	return true, nil
}

func (r Restorer) clearOld() error {
	err := os.RemoveAll(r.chainPath)
	if err != nil {
		return fmt.Errorf("failed to clear old chain path: %v", err)
	}
	err = os.RemoveAll(r.statePath)
	if err != nil {
		return fmt.Errorf("failed to clear old state path: %v", err)
	}
	return nil
}

func (r Restorer) downloadState() error {

	pre := fmt.Sprintf("%v/statedb/MANIFEST", r.heightToRestore)
	listParams := &s3.ListObjectsV2Input{
		Bucket: aws.String(r.s3bucket),
		Prefix: aws.String(pre),
	}
	listResult, err := r.s3.ListObjectsV2(listParams)
	if err != nil {
		return fmt.Errorf("failed to retrieve list of S3 objects: %v", err)
	}
	if len(listResult.Contents) <= 1 {
		return fmt.Errorf("failed to find state db in S3 (key = %v)", pre)
	}
	downloadParams := &s3.GetObjectInput{
		Bucket: aws.String(r.s3bucket),
	}
	err = os.Mkdir(r.statePath, 0777)
	if err != nil {
		return fmt.Errorf("failed to create state dir: %v", err)
	}
	for _, obj := range listResult.Contents {
		downloadParams.Key = obj.Key
		stateFile, err := r.s3.GetObject(downloadParams)
		if err != nil {
			return fmt.Errorf("failed to download S3 objects (height=%v, key=%v): %v", r.heightToRestore, obj.Key, err)
		}
		f := strings.Split(*obj.Key, "/")
		fileName := f[len(f)-1]
		file, err := os.Create(fmt.Sprintf("/Users/bittelc/go/src/github.com/herdius/herdius-core/%v/%v", r.statePath, fileName))
		if err != nil {
			return fmt.Errorf("failed to create state file %v: %v", fileName, err)
		}
		defer file.Close()
		body := make([]byte, *stateFile.ContentLength)
		_, err = stateFile.Body.Read(body)
		file.Write(body)
		log.Printf("successfully wrote to %v", fileName)
	}
	return nil
}

func (r Restorer) downloadChain() (*[]protobuf.BaseBlock, error) {
	listParams := &s3.ListObjectsV2Input{
		Bucket:  aws.String(r.s3bucket),
		Prefix:  aws.String("0/blocks/"),
		MaxKeys: aws.Int64(1),
	}
	listResult, err := r.s3.ListObjectsV2(listParams)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve list of S3 objects: %v", err)
	}
	if len(listResult.Contents) != 1 {
		return nil, fmt.Errorf("failed to find base block in S3 (block height = 0)")
	}
	log.Printf("root base block: %+v", *listResult.Contents[0].Key)

	key := *listResult.Contents[0].Key
	downloadParams := &s3.GetObjectInput{
		Bucket: aws.String(r.s3bucket),
		Key:    aws.String(key),
	}
	baseBlocks := &[]protobuf.BaseBlock{}

	for i := 0; i < r.heightToRestore; i++ {
		downloadParams.Key = aws.String(key)
		downResult, err := r.s3.GetObject(downloadParams)
		if err != nil {
			return nil, fmt.Errorf("failed to download S3 objects (height=%v, key=%v): %v", i, key, err)
		}

		///////////
		//_, err = downResult.Body.Read(content)
		baseBlock := protobuf.BaseBlock{}
		dec := json.NewDecoder(downResult.Body)
		err = dec.Decode(&baseBlock)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal S3 object into baseblock (height=%v, key=%v): %v", i, key, err)
		}
		spew.Dump(baseBlock)
		////////
		*baseBlocks = append(*baseBlocks, baseBlock)

		key, err = r.getKeyFromDownload(i+1, downResult)
		if err != nil {
			return nil, fmt.Errorf("failed to get key from prior block download (height=%v): %v", i, err)
		}

	}
	return baseBlocks, nil
}

func (r Restorer) replayChain(blocks *[]protobuf.BaseBlock) error {
	log.Println("replaying chain, number of blocks:", len(*blocks))
	chain := blockchain.Service{}
	for _, block := range *blocks {
		log.Printf("content: %+v", block)
		err := chain.AddBaseBlock(&block)
		if err != nil {
			return fmt.Errorf("couldn't add base block to chain: %v", err)
		}
	}
	return nil
}

func (r Restorer) getKeyFromDownload(i int, obj *s3.GetObjectOutput) (string, error) {
	log.Println("body content length:", *obj.ContentLength)
	body := make([]byte, *obj.ContentLength)
	_, err := obj.Body.Read(body)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("failed read body of s3 object output (i=%v): %v", i, err)
	}

	type b struct {
		Header struct {
			BlockID struct {
				BlockHash string `json:"blockHash"`
			} `json:"block_ID"`
		} `json:"header"`
	}
	var block b
	err = json.Unmarshal(body, &block)
	if err != nil {
		return "", fmt.Errorf("failed to parse json from request body (i=%v): %v", i, err)
	}
	if block.Header.BlockID.BlockHash == "" {
		return "", fmt.Errorf("request body json contains no blockhash (i=%v)", i)
	}
	log.Printf("block parsed: %+v", block)
	//hash := block.Header.BlockID.BlockHash

	// TODO ABOVE GETS HASH VALUE
	// TODO BELOW IS WORKAROUND, AS FILES ARE NOT CURRENTLY STORED WITH CORRECT NAME
	listParams := &s3.ListObjectsV2Input{
		Bucket:  aws.String(r.s3bucket),
		Prefix:  aws.String(fmt.Sprintf("%v/blocks/", i)),
		MaxKeys: aws.Int64(1),
	}
	listResult, err := r.s3.ListObjectsV2(listParams)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve list of S3 objects: %v", err)
	}
	if len(listResult.Contents) != 1 {
		return "", fmt.Errorf("failed to find base block in S3 (block height = 0)")
	}

	return *listResult.Contents[0].Key, nil

}
