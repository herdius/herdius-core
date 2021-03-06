// Modified for Herdius Requirements
// Original Copyright (c) 2018 Perlin Network
// https://github.com/perlin-network/noise/blob/master/LICENSE

package network

import (
	"bufio"
	"context"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/herdius/herdius-core/blockchain/protobuf"
	"github.com/herdius/herdius-core/p2p/crypto"
	"github.com/herdius/herdius-core/p2p/log"
	"github.com/herdius/herdius-core/p2p/network/transport"
	"github.com/herdius/herdius-core/p2p/peer"
	"github.com/herdius/herdius-core/p2p/types/opcode"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

const (
	defaultConnectionTimeout = 60 * time.Second
	defaultReceiveWindowSize = 4096
	defaultSendWindowSize    = 4096
	defaultWriteBufferSize   = 4096
	defaultWriteFlushLatency = 50 * time.Millisecond
	defaultWriteTimeout      = 3 * time.Second
	defaultaddress           = "tcp://127.0.0.1:2001"
)

var contextPool = sync.Pool{
	New: func() interface{} {
		return new(PluginContext)
	},
}

var (
	_ NetworkInterface = (*Network)(nil)
)

// Network represents the current networking state for this node.
type Network struct {
	opts options

	// Node's keypair.
	keys *crypto.KeyPair

	// Full address to listen on. `protocol://host:port`
	Address string

	// Map of plugins registered to the network.
	// map[string]Plugin
	plugins *PluginList

	// Node's cryptographic ID.
	ID peer.ID

	// Map of connection addresses (string) <-> *network.PeerClient
	// so that the Network doesn't dial multiple times to the same ip
	peers *sync.Map

	//RecvQueue chan *protobuf.Message

	// Map of connection addresses (string) <-> *ConnState
	connections *sync.Map

	// Map of protocol addresses (string) <-> *transport.Layer
	transports *sync.Map

	// listeningCh will block a goroutine until this node is listening for peers.
	listeningCh chan struct{}

	// <-kill will begin the server shutdown process
	kill chan struct{}
}

// options for network struct
type options struct {
	connectionTimeout time.Duration
	signaturePolicy   crypto.SignaturePolicy
	hashPolicy        crypto.HashPolicy
	recvWindowSize    int
	sendWindowSize    int
	writeBufferSize   int
	writeFlushLatency time.Duration
	writeTimeout      time.Duration
	address           string
}

// ConnState represents a connection.
type ConnState struct {
	conn         net.Conn
	writer       *bufio.Writer
	messageNonce uint64
	writerMutex  *sync.Mutex
}

// Init starts all network I/O workers.
func (n *Network) Init() {
	// Spawn write flusher.
	go n.flushLoop()
}

func (n *Network) flushLoop() {
	t := time.NewTicker(n.opts.writeFlushLatency)
	defer t.Stop()
	for {
		select {
		case <-n.kill:
			return
		case <-t.C:
			n.connections.Range(func(key, value interface{}) bool {
				if state, ok := value.(*ConnState); ok {
					state.writerMutex.Lock()
					if err := state.writer.Flush(); err != nil {
						log.Warn().Err(err).Msg("")
					}
					state.writerMutex.Unlock()
				}
				return true
			})
		}
	}
}

// GetKeys returns the keypair for this network
func (n *Network) GetKeys() *crypto.KeyPair {
	return n.keys
}

func (n *Network) dispatchMessage(client *PeerClient, msg *protobuf.Message) {
	if !client.IsIncomingReady() {
		return
	}
	var ptr proto.Message
	// unmarshal message based on specified opcode
	code := opcode.Opcode(msg.Opcode)
	switch code {
	case opcode.BytesCode:
		ptr = new(protobuf.Bytes)
	case opcode.PingCode:
		ptr = new(protobuf.Ping)
	case opcode.PongCode:
		ptr = new(protobuf.Pong)
	case opcode.LookupNodeRequestCode:
		ptr = new(protobuf.LookupNodeRequest)
	case opcode.LookupNodeResponseCode:
		ptr = new(protobuf.LookupNodeResponse)
	case opcode.UnregisteredCode:
		log.Error().Msg("network: message received had no opcode")
		return
	default:
		var err error
		ptr, err = opcode.GetMessageType(code)
		if err != nil {
			log.Error().Err(err).Msg("network: received message opcode is not registered")
			return
		}
	}

	if len(msg.Message) > 0 {
		if err := proto.Unmarshal(msg.Message, ptr); err != nil {
			log.Error().Msgf("%v", err)
			return
		}
	}

	if msg.RequestNonce > 0 && msg.ReplyFlag {
		if _state, exists := client.Requests.Load(msg.RequestNonce); exists {
			state := _state.(*RequestState)
			select {
			case state.data <- ptr:
			case <-state.closeSignal:
			}
			return
		}
	}

	switch msgRaw := ptr.(type) {
	case *protobuf.Bytes:
		client.handleBytes(msgRaw.Data)
	default:
		ctx := contextPool.Get().(*PluginContext)
		ctx.client = client
		ctx.message = msgRaw
		ctx.nonce = msg.RequestNonce

		go func() {
			// Execute 'on receive message' callback for all plugins.
			n.plugins.Each(func(plugin PluginInterface) {
				if err := plugin.Receive(ctx); err != nil {
					log.Error().Err(err).Msg("")
				}
			})

			contextPool.Put(ctx)
		}()
	}
}

var SupervisorListen net.Listener

// Listen starts listening for peers on a port.
func (n *Network) Listen() {
	// Handle 'network starts listening' callback for plugins.
	n.plugins.Each(func(plugin PluginInterface) {
		plugin.Startup(n)
	})

	// Handle 'network stops listening' callback for plugins.
	defer func() {
		n.plugins.Each(func(plugin PluginInterface) {
			plugin.Cleanup(n)
		})
	}()

	addrInfo, err := ParseAddress(n.Address)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	var listener net.Listener

	if t, exists := n.transports.Load(addrInfo.Protocol); exists {
		listener, err = t.(transport.Layer).Listen(int(addrInfo.Port))
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
	} else {
		err := errors.New("network: invalid protocol " + addrInfo.Protocol)
		log.Fatal().Err(err).Msg("")
	}

	n.startListening()
	SupervisorListen = listener
	log.Info().
		Str("address", n.Address).
		Msg("Listening for peers.")

	// handle server shutdowns
	go func() {
		select {
		case <-n.kill:
			// cause listener.Accept() to stop blocking so it can continue the loop
			listener.Close()
		}
	}()

	// Handle new clients.
	for {
		if conn, err := listener.Accept(); err == nil {
			go n.Accept(conn)
		} else {
			// if the Shutdown flag is set, no need to continue with the for loop
			select {
			case <-n.kill:
				log.Info().Msgf("Shutting down server %s.", n.Address)
				return
			default:
				log.Error().Msgf("%v", err)
			}
		}
	}
}

// Client either creates or returns a cached peer client given its host address.
func (n *Network) Client(address string) (*PeerClient, error) {
	address, err := ToUnifiedAddress(address)

	if err != nil {
		return nil, err
	}

	if address == n.Address {
		return nil, errors.New("network: peer should not dial itself")
	}

	clientNew, err := createPeerClient(n, address)
	if err != nil {
		return nil, err
	}

	c, exists := n.peers.LoadOrStore(address, clientNew)
	if exists {
		client := c.(*PeerClient)

		if !client.IsOutgoingReady() {
			return nil, errors.New("network: peer failed to connect")
		}

		return client, nil
	}

	client := c.(*PeerClient)
	defer func() {
		client.setOutgoingReady()
	}()

	conn, err := n.Dial(address)
	if err != nil {
		n.peers.Delete(address)
		return nil, err
	}

	n.connections.Store(address, &ConnState{
		conn:        conn,
		writer:      bufio.NewWriterSize(conn, n.opts.writeBufferSize),
		writerMutex: new(sync.Mutex),
	})

	client.Init()

	return client, nil
}

// ConnectionStateExists returns true if network has a connection on a given address.
func (n *Network) ConnectionStateExists(address string) bool {
	_, ok := n.connections.Load(address)
	return ok
}

// ConnectionState returns a connections state for current address.
func (n *Network) ConnectionState(address string) (*ConnState, bool) {
	conn, ok := n.connections.Load(address)
	if !ok {
		return nil, false
	}
	return conn.(*ConnState), true
}

// GetNetConnection returns a network connection object for current address.
func (n *Network) GetNetConnection(address string) net.Conn {
	conn, ok := n.connections.Load(address)
	if !ok {
		return nil
	}
	netConn := *(conn.(*ConnState))
	return netConn.conn
}

// startListening will start node for listening for new peers.
func (n *Network) startListening() {
	close(n.listeningCh)
}

// BlockUntilListening blocks until this node is listening for new peers.
func (n *Network) BlockUntilListening() {
	<-n.listeningCh
}

// Bootstrap with a number of peers and commence a handshake.
func (n *Network) Bootstrap(addresses ...string) {
	n.BlockUntilListening()

	addresses = FilterPeers(n.Address, addresses)

	for _, address := range addresses {
		client, err := n.Client(address)
		if err != nil {
			// TODO: Need to display a graceful error message
			// when connection is disconnected
			//log.Error().Err(err).Msg("")
			continue
		}

		err = client.Tell(context.Background(), &protobuf.Ping{})
		if err != nil {
			continue
		}
	}
}

// Dial establishes a bidirectional connection to an address, and additionally handshakes with said address.
func (n *Network) Dial(address string) (net.Conn, error) {
	addrInfo, err := ParseAddress(address)
	if err != nil {
		return nil, err
	}

	if addrInfo.Host != "127.0.0.1" {
		host, err := ParseAddress(n.Address)
		if err != nil {
			return nil, err
		}
		// check if dialing address is same as its own IP
		if addrInfo.Host == host.Host {
			addrInfo.Host = "127.0.0.1"
		}
	}

	// Choose scheme.
	t, exists := n.transports.Load(addrInfo.Protocol)
	if !exists {
		err := errors.New("network: invalid protocol " + addrInfo.Protocol)
		log.Fatal().Err(err).Msg("")
	}

	var conn net.Conn

	conn, err = t.(transport.Layer).Dial(addrInfo.HostPort())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// Accept handles peer registration and processes incoming message streams.
func (n *Network) Accept(incoming net.Conn) {

	var client *PeerClient

	recvWindow := NewRecvWindow(n.opts.recvWindowSize)

	// Cleanup connections when we are done with them.
	defer func() {
		time.Sleep(1 * time.Second)

		if client != nil {
			client.Close()
		}

		if incoming != nil {
			incoming.Close()
		}
	}()

	for {

		//Add receiveChildBlock for the child
		msg, err := n.receiveMessage(incoming)
		if err != nil {
			if err != errEmptyMsg {
				log.Error().Msgf("%v", err)
			}
			break
		}

		// Initialize client if not exists.
		if client == nil {
			client, err = n.Client(msg.Sender.Address)
			if err != nil {
				return
			}
		}

		client.Do(func() {
			client.ID = (*peer.ID)(msg.Sender)

			if !n.ConnectionStateExists(client.ID.Address) {
				err = errors.New("network: failed to load session")
			}

			client.setIncomingReady()
		})

		if err != nil {
			log.Error().Err(err).Msg("network: error initializing client")
			return
		}

		go func() {
			// Peer sent message with a completely different ID. Disconnect.
			if !client.ID.Equals(peer.ID(*msg.Sender)) {
				log.Error().
					Interface("peer_id", peer.ID(*msg.Sender)).
					Interface("client_id", client.ID).
					Msg("Message signed by peer does not match client ID.")
				return
			}

			recvWindow.Push(msg.MessageNonce, msg)

			ready := recvWindow.Pop()
			for _, msg := range ready {
				msg := msg
				client.Submit(func() {
					n.dispatchMessage(client, msg.(*protobuf.Message))
				})
			}
		}()
	}
}

// Plugin returns a plugins proxy interface should it be registered with the
// network. The second returning parameter is false otherwise.
//
// Example: network.Plugin((*Plugin)(nil))
func (n *Network) Plugin(key interface{}) (PluginInterface, bool) {
	return n.plugins.Get(key)
}

// PrepareMessage marshals a message into a *protobuf.Message and signs it with this
// nodes private key. Errors if the message is null.
func (n *Network) PrepareMessage(ctx context.Context, message proto.Message) (*protobuf.Message, error) {
	if message == nil {
		return nil, errors.New("network: message is null")
	}

	opcode, err := opcode.GetOpcode(message)
	if err != nil {
		return nil, err
	}

	raw, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}

	id := protobuf.ID(n.ID)

	msg := &protobuf.Message{
		Message: raw,
		Opcode:  uint32(opcode),
		Sender:  &id,
	}

	if GetSignMessage(ctx) {
		signature, err := n.keys.Sign(
			n.opts.signaturePolicy,
			n.opts.hashPolicy,
			SerializeMessage(&id, raw),
		)
		if err != nil {
			return nil, err
		}
		msg.Signature = signature
	}

	return msg, nil
}

// Write asynchronously sends a message to a denoted target address.
func (n *Network) Write(address string, message *protobuf.Message) error {
	state, ok := n.ConnectionState(address)
	if !ok {
		return errors.New("network: connection does not exist")
	}

	message.MessageNonce = atomic.AddUint64(&state.messageNonce, 1)

	state.conn.SetWriteDeadline(time.Now().Add(n.opts.writeTimeout))

	err := n.sendMessage(state.writer, message, state.writerMutex)
	if err != nil {
		return err
	}
	return nil
}

// PrepareChildBlock marshals a message into a *proto3.ChildBlock and signs it with this
// nodes private key. Errors if the message is null.
func (n *Network) PrepareChildBlock(ctx context.Context, message proto.Message) (*protobuf.ChildBlock, error) {
	if message == nil {
		return nil, errors.New("network: message is null")
	}

	// raw, err := proto.Marshal(message)
	// if err != nil {
	// 	return nil, err
	// }

	//id := protobuf.ID(n.PID)
	//fmt.Println(raw)
	cb := &protobuf.ChildBlock{}
	if GetSignMessage(ctx) {
	}
	// if GetSignMessage(ctx) {
	// 	signature, err := n.keys.Sign(
	// 		n.opts.signaturePolicy,
	// 		n.opts.hashPolicy,
	// 		//SerializeMessage(&id, raw),
	// 		nil,
	// 	)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	//msg.Signature = signature
	// }

	return cb, nil
}

// WriteChildBlock asynchronously sends a message to a denoted target address.
func (n *Network) WriteChildBlock(address string, cb *protobuf.ChildBlock) error {
	state, ok := n.ConnectionState(address)

	if !ok {
		return errors.New("network: connection does not exist")
	}
	state.conn.SetWriteDeadline(time.Now().Add(n.opts.writeTimeout))

	// err := n.sendChildBlock(state.writer, cb, state.writerMutex)
	// if err != nil {
	// 	return err
	// }
	return nil
}

// Broadcast asynchronously broadcasts a message to all peer clients.
func (n *Network) Broadcast(ctx context.Context, message proto.Message) {
	signed, err := n.PrepareMessage(ctx, message)
	if err != nil {
		log.Error().Err(err).Msg("network: failed to broadcast message")
		return
	}
	//fmt.Println(n.peers)
	n.eachPeer(func(client *PeerClient) bool {

		err := n.Write(client.Address, signed)
		if err != nil {
			log.Warn().
				Err(err).
				Interface("peer_id", client.ID).
				Msg("failed to send message to peer")
		}
		return true
	})

}

/* func (n *Network) Broadcast(ctx context.Context, message proto.Message) {

	signed, err := n.PrepareMessage(ctx, message)
	if err != nil {
		log.Error().Err(err).Msg("network: failed to broadcast message")
		return
	}
	//fmt.Println(n.peers)
	n.eachPeer(func(client *PeerClient) bool {
		err := n.Write(client.Address, signed)
		if err != nil {
			log.Warn().
				Err(err).
				Interface("peer_id", client.ID).
				Msg("failed to send message to peer")
		}
		return true
	})
} */

// SendByID is for testing and it should be deleted
func (n *Network) SendByID(ctx context.Context, address string, message proto.Message) {
	signed, err := n.PrepareMessage(ctx, message)
	if err != nil {
		log.Error().Err(err).Msg("network: failed to broadcast message")
		return
	}

	err = n.Write(address, signed)
	if err != nil {
		log.Warn().
			Err(err).
			Interface("peer_id", n.ID).
			Msg("failed to send message to peer")
	}
}

//SendChildBlockByAddress ...
func (n *Network) SendChildBlockByAddress(ctx context.Context, address string, cb protobuf.ChildBlock) {
	err := n.WriteChildBlock(address, &cb)

	if err != nil {
		log.Warn().
			Err(err).
			Interface("peer_id", n.ID).
			Msg("failed to send message to peer")
	}
}

// BroadcastChildBlock asynchronously broadcasts the child block to all peer validators.
// tcp://127.0.0.1:3001
func (n *Network) BroadcastChildBlock(ctx context.Context, message proto.Message) {
	cb, err := n.PrepareChildBlock(ctx, message)
	if err != nil {
		log.Error().Err(err).Msg("network: failed to broadcast message")
		return
	}

	n.eachPeer(func(client *PeerClient) bool {
		err := n.WriteChildBlock(client.Address, cb)
		if err != nil {
			log.Warn().
				Err(err).
				Interface("peer_id", client.ID).
				Msg("failed to send message to peer")
		}
		return true
	})
}

// BroadcastByAddresses broadcasts a message to a set of peer clients denoted by their addresses.
func (n *Network) BroadcastByAddresses(ctx context.Context, message proto.Message, addresses ...string) {
	signed, err := n.PrepareMessage(ctx, message)
	if err != nil {
		return
	}

	for _, address := range addresses {
		n.Write(address, signed)
	}
}

// BroadcastChildBlockByAddresses broadcasts a message to a set of peer clients denoted by their addresses.
func (n *Network) BroadcastChildBlockByAddresses(ctx context.Context, message proto.Message, addresses ...string) {
	cb, err := n.PrepareChildBlock(ctx, message)
	if err != nil {
		return
	}

	for _, address := range addresses {
		n.WriteChildBlock(address, cb)
	}
}

// BroadcastByIDs broadcasts a message to a set of peer clients denoted by their peer IDs.
func (n *Network) BroadcastByIDs(ctx context.Context, message proto.Message, ids ...peer.ID) {
	signed, err := n.PrepareMessage(ctx, message)
	if err != nil {
		return
	}

	for _, id := range ids {
		n.Write(id.Address, signed)
	}
}

// BroadcastRandomly asynchronously broadcasts a message to random selected K peers.
// Does not guarantee broadcasting to exactly K peers.
func (n *Network) BroadcastRandomly(ctx context.Context, message proto.Message, K int) {
	var addresses []string

	n.eachPeer(func(client *PeerClient) bool {
		addresses = append(addresses, client.Address)

		// Limit total amount of addresses in case we have a lot of peers.
		return len(addresses) <= K*3
	})

	// Flip a coin and shuffle :).
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})

	if len(addresses) < K {
		K = len(addresses)
	}

	n.BroadcastByAddresses(ctx, message, addresses[:K]...)
}

// Close shuts down the entire network.
func (n *Network) Close() {
	close(n.kill)

	n.eachPeer(func(client *PeerClient) bool {
		client.Close()
		return true
	})
}

func (n *Network) eachPeer(fn func(client *PeerClient) bool) {
	n.peers.Range(func(_, value interface{}) bool {
		client := value.(*PeerClient)
		return fn(client)
	})
}
