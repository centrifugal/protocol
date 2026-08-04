package protocol

import "sync"

// ReplyPoolCollection is a set of pools of Reply objects, one pool per result
// type. Reusing Reply objects keeps a server from allocating an envelope for
// every command it answers.
//
// Use the shared ReplyPool rather than creating a collection directly.
type ReplyPoolCollection struct {
	connectReplyPool       sync.Pool
	subscribeReplyPool     sync.Pool
	unsubscribeReplyPool   sync.Pool
	publishReplyPool       sync.Pool
	rpcReplyPool           sync.Pool
	presenceReplyPool      sync.Pool
	presenceStatsReplyPool sync.Pool
	historyReplyPool       sync.Pool
	refreshReplyPool       sync.Pool
	subRefreshReplyPool    sync.Pool
}

// ReplyPool is the shared collection of Reply pools.
//
// Every Acquire method returns a Reply with the corresponding result field set,
// and with Id and Error zeroed. Once the Reply is encoded and no longer needed,
// pass it to the matching Release method – after that neither the Reply nor the
// result it referenced must be used.
//
// Release clears the whole envelope, not just the result field: a pooled Reply
// is handed to a different connection next, so any Id or Error left on it would
// be sent to that connection instead.
//
//goland:noinspection GoUnusedGlobalVariable
var ReplyPool = &ReplyPoolCollection{}

// AcquireConnectReply takes a Reply from the pool and sets the given ConnectResult on it.
func (p *ReplyPoolCollection) AcquireConnectReply(result *ConnectResult) *Reply {
	r := p.connectReplyPool.Get()
	if r == nil {
		return &Reply{
			Connect: result,
		}
	}
	reply := r.(*Reply)
	reply.Connect = result
	return reply
}

// ReleaseConnectReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseConnectReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Connect = nil
	p.connectReplyPool.Put(r)
}

// AcquireSubscribeReply takes a Reply from the pool and sets the given SubscribeResult on it.
func (p *ReplyPoolCollection) AcquireSubscribeReply(result *SubscribeResult) *Reply {
	r := p.subscribeReplyPool.Get()
	if r == nil {
		return &Reply{
			Subscribe: result,
		}
	}
	reply := r.(*Reply)
	reply.Subscribe = result
	return reply
}

// ReleaseSubscribeReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseSubscribeReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Subscribe = nil
	p.subscribeReplyPool.Put(r)
}

// AcquireUnsubscribeReply takes a Reply from the pool and sets the given UnsubscribeResult on it.
func (p *ReplyPoolCollection) AcquireUnsubscribeReply(result *UnsubscribeResult) *Reply {
	r := p.unsubscribeReplyPool.Get()
	if r == nil {
		return &Reply{
			Unsubscribe: result,
		}
	}
	reply := r.(*Reply)
	reply.Unsubscribe = result
	return reply
}

// ReleaseUnsubscribeReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseUnsubscribeReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Unsubscribe = nil
	p.unsubscribeReplyPool.Put(r)
}

// AcquirePublishReply takes a Reply from the pool and sets the given PublishResult on it.
func (p *ReplyPoolCollection) AcquirePublishReply(result *PublishResult) *Reply {
	r := p.publishReplyPool.Get()
	if r == nil {
		return &Reply{
			Publish: result,
		}
	}
	reply := r.(*Reply)
	reply.Publish = result
	return reply
}

// ReleasePublishReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleasePublishReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Publish = nil
	p.publishReplyPool.Put(r)
}

// AcquireRPCReply takes a Reply from the pool and sets the given RPCResult on it.
func (p *ReplyPoolCollection) AcquireRPCReply(result *RPCResult) *Reply {
	r := p.rpcReplyPool.Get()
	if r == nil {
		return &Reply{
			Rpc: result,
		}
	}
	reply := r.(*Reply)
	reply.Rpc = result
	return reply
}

// ReleaseRPCReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseRPCReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Rpc = nil
	p.rpcReplyPool.Put(r)
}

// AcquirePresenceReply takes a Reply from the pool and sets the given PresenceResult on it.
func (p *ReplyPoolCollection) AcquirePresenceReply(result *PresenceResult) *Reply {
	r := p.presenceReplyPool.Get()
	if r == nil {
		return &Reply{
			Presence: result,
		}
	}
	reply := r.(*Reply)
	reply.Presence = result
	return reply
}

// ReleasePresenceReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleasePresenceReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Presence = nil
	p.presenceReplyPool.Put(r)
}

// AcquirePresenceStatsReply takes a Reply from the pool and sets the given PresenceStatsResult on it.
func (p *ReplyPoolCollection) AcquirePresenceStatsReply(result *PresenceStatsResult) *Reply {
	r := p.presenceStatsReplyPool.Get()
	if r == nil {
		return &Reply{
			PresenceStats: result,
		}
	}
	reply := r.(*Reply)
	reply.PresenceStats = result
	return reply
}

// ReleasePresenceStatsReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleasePresenceStatsReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.PresenceStats = nil
	p.presenceStatsReplyPool.Put(r)
}

// AcquireHistoryReply takes a Reply from the pool and sets the given HistoryResult on it.
func (p *ReplyPoolCollection) AcquireHistoryReply(result *HistoryResult) *Reply {
	r := p.historyReplyPool.Get()
	if r == nil {
		return &Reply{
			History: result,
		}
	}
	reply := r.(*Reply)
	reply.History = result
	return reply
}

// ReleaseHistoryReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseHistoryReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.History = nil
	p.historyReplyPool.Put(r)
}

// AcquireRefreshReply takes a Reply from the pool and sets the given RefreshResult on it.
func (p *ReplyPoolCollection) AcquireRefreshReply(result *RefreshResult) *Reply {
	r := p.refreshReplyPool.Get()
	if r == nil {
		return &Reply{
			Refresh: result,
		}
	}
	reply := r.(*Reply)
	reply.Refresh = result
	return reply
}

// ReleaseRefreshReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseRefreshReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.Refresh = nil
	p.refreshReplyPool.Put(r)
}

// AcquireSubRefreshReply takes a Reply from the pool and sets the given SubRefreshResult on it.
func (p *ReplyPoolCollection) AcquireSubRefreshReply(result *SubRefreshResult) *Reply {
	r := p.subRefreshReplyPool.Get()
	if r == nil {
		return &Reply{
			SubRefresh: result,
		}
	}
	reply := r.(*Reply)
	reply.SubRefresh = result
	return reply
}

// ReleaseSubRefreshReply clears r and returns it to the pool. Neither r nor the result
// it referenced must be used after this call.
func (p *ReplyPoolCollection) ReleaseSubRefreshReply(r *Reply) {
	r.Id = 0
	r.Error = nil
	r.SubRefresh = nil
	p.subRefreshReplyPool.Put(r)
}
