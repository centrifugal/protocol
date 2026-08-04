package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A pooled Reply is handed to a different connection next, so Release must clear
// the whole envelope - a leftover Id or Error would be sent to that connection.
func TestReplyPool_ReleaseClearsEnvelope(t *testing.T) {
	r1 := ReplyPool.AcquireConnectReply(&ConnectResult{Client: "client-A"})
	r1.Id = 42
	r1.Error = &Error{Code: 109, Message: "token expired"}
	ReplyPool.ReleaseConnectReply(r1)

	r2 := ReplyPool.AcquireConnectReply(&ConnectResult{Client: "client-B"})
	defer ReplyPool.ReleaseConnectReply(r2)
	require.Zero(t, r2.Id, "Id from a previous user must not leak")
	require.Nil(t, r2.Error, "Error from a previous user must not leak")
	require.Equal(t, "client-B", r2.Connect.Client)
}

// Every Release method must clear the envelope, not only its own result field.
func TestReplyPool_AllReleasersClearEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		acquire  func() *Reply
		release  func(*Reply)
		resultOf func(*Reply) any
	}{
		{"connect", func() *Reply { return ReplyPool.AcquireConnectReply(&ConnectResult{}) }, ReplyPool.ReleaseConnectReply, func(r *Reply) any { return r.Connect }},
		{"subscribe", func() *Reply { return ReplyPool.AcquireSubscribeReply(&SubscribeResult{}) }, ReplyPool.ReleaseSubscribeReply, func(r *Reply) any { return r.Subscribe }},
		{"unsubscribe", func() *Reply { return ReplyPool.AcquireUnsubscribeReply(&UnsubscribeResult{}) }, ReplyPool.ReleaseUnsubscribeReply, func(r *Reply) any { return r.Unsubscribe }},
		{"publish", func() *Reply { return ReplyPool.AcquirePublishReply(&PublishResult{}) }, ReplyPool.ReleasePublishReply, func(r *Reply) any { return r.Publish }},
		{"rpc", func() *Reply { return ReplyPool.AcquireRPCReply(&RPCResult{}) }, ReplyPool.ReleaseRPCReply, func(r *Reply) any { return r.Rpc }},
		{"presence", func() *Reply { return ReplyPool.AcquirePresenceReply(&PresenceResult{}) }, ReplyPool.ReleasePresenceReply, func(r *Reply) any { return r.Presence }},
		{"presence stats", func() *Reply { return ReplyPool.AcquirePresenceStatsReply(&PresenceStatsResult{}) }, ReplyPool.ReleasePresenceStatsReply, func(r *Reply) any { return r.PresenceStats }},
		{"history", func() *Reply { return ReplyPool.AcquireHistoryReply(&HistoryResult{}) }, ReplyPool.ReleaseHistoryReply, func(r *Reply) any { return r.History }},
		{"refresh", func() *Reply { return ReplyPool.AcquireRefreshReply(&RefreshResult{}) }, ReplyPool.ReleaseRefreshReply, func(r *Reply) any { return r.Refresh }},
		{"sub refresh", func() *Reply { return ReplyPool.AcquireSubRefreshReply(&SubRefreshResult{}) }, ReplyPool.ReleaseSubRefreshReply, func(r *Reply) any { return r.SubRefresh }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.acquire()
			r.Id = 7
			r.Error = &Error{Code: 100, Message: "boom"}
			tt.release(r)
			require.Zero(t, r.Id)
			require.Nil(t, r.Error)
			require.Nil(t, tt.resultOf(r))
		})
	}
}
