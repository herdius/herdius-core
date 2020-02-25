package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBalance(t *testing.T) {
	es := newEthSyncer()
	_, err := es.getBalance("0xE4148EAa01846729B6596d3939cc2B342CB4701D")
	assert.Equal(t, err, nil)
}
