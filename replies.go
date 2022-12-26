package protocol

import "sync"

type replyPool struct {
	connectReplyPool     sync.Pool
	subscribeReplyPool   sync.Pool
	unsubscribeReplyPool sync.Pool
	publishReplyPool     sync.Pool
	rpcReplyPool         sync.Pool
}

//goland:noinspection GoUnusedGlobalVariable
var ReplyPool = &replyPool{}

func (p *replyPool) AcquireConnectReply(result *ConnectResult) *Reply {
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

func (p *replyPool) ReleaseConnectReply(r *Reply) {
	r.Connect = nil
	p.connectReplyPool.Put(r)
}

func (p *replyPool) AcquireSubscribeReply(result *SubscribeResult) *Reply {
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

func (p *replyPool) ReleaseSubscribeReply(r *Reply) {
	r.Subscribe = nil
	p.subscribeReplyPool.Put(r)
}

func (p *replyPool) AcquireUnsubscribeReply(result *UnsubscribeResult) *Reply {
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

func (p *replyPool) ReleaseUnsubscribeReply(r *Reply) {
	r.Unsubscribe = nil
	p.unsubscribeReplyPool.Put(r)
}

func (p *replyPool) AcquirePublishReply(result *PublishResult) *Reply {
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

func (p *replyPool) ReleasePublishReply(r *Reply) {
	r.Publish = nil
	p.publishReplyPool.Put(r)
}

func (p *replyPool) AcquireRPCReply(result *RPCResult) *Reply {
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

func (p *replyPool) ReleaseRPCReply(r *Reply) {
	r.Rpc = nil
	p.rpcReplyPool.Put(r)
}
