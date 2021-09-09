package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushHelpers(t *testing.T) {
	msg := newMessagePush(Raw("{}"))
	require.NotNil(t, msg)
	msg = newJoinPush("test", Raw("{}"))
	require.NotNil(t, msg)
	msg = newLeavePush("test", Raw("{}"))
	require.NotNil(t, msg)
	msg = newPublicationPush("test", Raw("{}"))
	require.NotNil(t, msg)
	msg = newSubscribePush("test", Raw("{}"))
	require.NotNil(t, msg)
	msg = newUnsubscribePush("test", Raw("{}"))
	require.NotNil(t, msg)
	msg = newConnectPush(Raw("{}"))
	require.NotNil(t, msg)
	msg = newDisconnectPush(Raw("{}"))
	require.NotNil(t, msg)
	msg = newRefreshPush(Raw("{}"))
	require.NotNil(t, msg)
}
