package client

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// A real GET_STATUS_NETWORK reply from a camera. Its field 14 wasn't in the
// schema, which is why the dashboard never had the WiFi name.
func TestNetworkStatusDecodes(t *testing.T) {
	raw, err := hex.DecodeString("0806101618c801721b08001207415250414e455418cbffffffffffffffff01200328ad2d")
	require.NoError(t, err)

	var res Response
	require.NoError(t, proto.Unmarshal(raw, &res))
	assert.Equal(t, RequestType_GET_STATUS_NETWORK, res.GetRequestType())

	network := res.GetNetworkStatus()
	require.NotNil(t, network)
	assert.Equal(t, "ARPANET", network.GetSsid())
	assert.Equal(t, int32(-53), network.GetRssi())
	assert.Equal(t, int32(5805), network.GetFrequency())
}
