// Struct definitions copied from ../../client.pb.go with everything but the
// types removed, see README.md. DO NOT EDIT.

package cfjsoncmp

import "google.golang.org/protobuf/runtime/protoimpl"

type Error struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          uint32                 `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Temporary     bool                   `protobuf:"varint,3,opt,name=temporary,proto3" json:"temporary,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type EmulationRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Node          string                 `protobuf:"bytes,1,opt,name=node,proto3" json:"node,omitempty"`
	Session       string                 `protobuf:"bytes,2,opt,name=session,proto3" json:"session,omitempty"`
	Data          Raw                    `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Command struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Id of command to let client match replies to commands.
	Id uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Client can send one of the following requests. Server will
	// only take the first non-null request out of these and may return an error if
	// client passed more than one request. We are not using oneof here due to JSON
	// interoperability concerns.
	Connect       *ConnectRequest       `protobuf:"bytes,4,opt,name=connect,proto3" json:"connect,omitempty"`
	Subscribe     *SubscribeRequest     `protobuf:"bytes,5,opt,name=subscribe,proto3" json:"subscribe,omitempty"`
	Unsubscribe   *UnsubscribeRequest   `protobuf:"bytes,6,opt,name=unsubscribe,proto3" json:"unsubscribe,omitempty"`
	Publish       *PublishRequest       `protobuf:"bytes,7,opt,name=publish,proto3" json:"publish,omitempty"`
	Presence      *PresenceRequest      `protobuf:"bytes,8,opt,name=presence,proto3" json:"presence,omitempty"`
	PresenceStats *PresenceStatsRequest `protobuf:"bytes,9,opt,name=presence_stats,json=presenceStats,proto3" json:"presence_stats,omitempty"`
	History       *HistoryRequest       `protobuf:"bytes,10,opt,name=history,proto3" json:"history,omitempty"`
	Ping          *PingRequest          `protobuf:"bytes,11,opt,name=ping,proto3" json:"ping,omitempty"`
	Send          *SendRequest          `protobuf:"bytes,12,opt,name=send,proto3" json:"send,omitempty"`
	Rpc           *RPCRequest           `protobuf:"bytes,13,opt,name=rpc,proto3" json:"rpc,omitempty"`
	Refresh       *RefreshRequest       `protobuf:"bytes,14,opt,name=refresh,proto3" json:"refresh,omitempty"`
	SubRefresh    *SubRefreshRequest    `protobuf:"bytes,15,opt,name=sub_refresh,json=subRefresh,proto3" json:"sub_refresh,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Reply struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Id will only be set to a value > 0 for replies to commands. For pushes
	// coming from server to client it has zero value.
	Id uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Error can only be set in replies to commands. For pushes it is never set.
	Error *Error `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	// ProtocolVersion2 server can send one of the following fields. We are not using
	// oneof here due to JSON interoperability concerns.
	Push          *Push                `protobuf:"bytes,4,opt,name=push,proto3" json:"push,omitempty"`
	Connect       *ConnectResult       `protobuf:"bytes,5,opt,name=connect,proto3" json:"connect,omitempty"`
	Subscribe     *SubscribeResult     `protobuf:"bytes,6,opt,name=subscribe,proto3" json:"subscribe,omitempty"`
	Unsubscribe   *UnsubscribeResult   `protobuf:"bytes,7,opt,name=unsubscribe,proto3" json:"unsubscribe,omitempty"`
	Publish       *PublishResult       `protobuf:"bytes,8,opt,name=publish,proto3" json:"publish,omitempty"`
	Presence      *PresenceResult      `protobuf:"bytes,9,opt,name=presence,proto3" json:"presence,omitempty"`
	PresenceStats *PresenceStatsResult `protobuf:"bytes,10,opt,name=presence_stats,json=presenceStats,proto3" json:"presence_stats,omitempty"`
	History       *HistoryResult       `protobuf:"bytes,11,opt,name=history,proto3" json:"history,omitempty"`
	Ping          *PingResult          `protobuf:"bytes,12,opt,name=ping,proto3" json:"ping,omitempty"`
	Rpc           *RPCResult           `protobuf:"bytes,13,opt,name=rpc,proto3" json:"rpc,omitempty"`
	Refresh       *RefreshResult       `protobuf:"bytes,14,opt,name=refresh,proto3" json:"refresh,omitempty"`
	SubRefresh    *SubRefreshResult    `protobuf:"bytes,15,opt,name=sub_refresh,json=subRefresh,proto3" json:"sub_refresh,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Push struct {
	state   protoimpl.MessageState `protogen:"open.v1"`
	Id      int64                  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"` // Optional numeric channel ID to avoid sending string channel in Push (bandwidth optimization).
	Channel string                 `protobuf:"bytes,2,opt,name=channel,proto3" json:"channel,omitempty"`
	// Server can push one of the following fields to the client. We are
	// not using oneof here due to JSON interoperability concerns.
	Pub           *Publication `protobuf:"bytes,4,opt,name=pub,proto3" json:"pub,omitempty"`
	Join          *Join        `protobuf:"bytes,5,opt,name=join,proto3" json:"join,omitempty"`
	Leave         *Leave       `protobuf:"bytes,6,opt,name=leave,proto3" json:"leave,omitempty"`
	Unsubscribe   *Unsubscribe `protobuf:"bytes,7,opt,name=unsubscribe,proto3" json:"unsubscribe,omitempty"`
	Message       *Message     `protobuf:"bytes,8,opt,name=message,proto3" json:"message,omitempty"`
	Subscribe     *Subscribe   `protobuf:"bytes,9,opt,name=subscribe,proto3" json:"subscribe,omitempty"`
	Connect       *Connect     `protobuf:"bytes,10,opt,name=connect,proto3" json:"connect,omitempty"`
	Disconnect    *Disconnect  `protobuf:"bytes,11,opt,name=disconnect,proto3" json:"disconnect,omitempty"`
	Refresh       *Refresh     `protobuf:"bytes,12,opt,name=refresh,proto3" json:"refresh,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Dictionary struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Identifier of dictionary content, client may cache by it.
	//
	// It is SHA-256 of the content, first 12 bytes, base64url without padding.
	//
	// The derivation is fixed rather than conventional, because a client that
	// keeps a dictionary between connections hashes the bytes it stored against
	// this id before reusing them. Storage shared with anything else on the
	// client's origin can be rewritten, and DEFLATE back references resolve into
	// the dictionary - so substituted content does not corrupt a frame, it makes
	// a genuine server frame decode into whatever the substituter chose. Only an
	// id the server issued detects that: a checksum the client stored alongside
	// would simply be rewritten with the bytes.
	//
	// A server identifying dictionaries some other way is not rejected. It has
	// every caching client quietly stop caching instead, which is worse.
	//
	// It also means one id can never name two different dictionaries, so a client
	// reconnecting to a node that built its own gets an exact match or a miss,
	// never the wrong bytes.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// The dictionary content, always DEFLATE compressed. Inflate it with no preset
	// dictionary before use.
	//
	// It is compressed because it arrives in the connect reply, which cannot be
	// compressed at the frame level - nothing is installed yet to compress it
	// against. Compressing the content instead is worth about 8x on JSON, and a
	// dictionary is sampled message text so it always compresses.
	//
	// data is set on Protobuf connections, where bytes travel directly. data_b64
	// is set on JSON connections, where a bytes field carries raw JSON and cannot
	// hold binary. Exactly one of them is set, and neither when the server is
	// naming a dictionary the client already holds.
	Data          Raw    `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	DataB64       string `protobuf:"bytes,3,opt,name=data_b64,json=dataB64,proto3" json:"data_b64,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type ClientInfo struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	User          string                 `protobuf:"bytes,1,opt,name=user,proto3" json:"user"`
	Client        string                 `protobuf:"bytes,2,opt,name=client,proto3" json:"client"`
	ConnInfo      Raw                    `protobuf:"bytes,3,opt,name=conn_info,json=connInfo,proto3" json:"conn_info,omitempty"`
	ChanInfo      Raw                    `protobuf:"bytes,4,opt,name=chan_info,json=chanInfo,proto3" json:"chan_info,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Publication struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          Raw                    `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`                                                                           // Data contains publication payload.
	Info          *ClientInfo            `protobuf:"bytes,5,opt,name=info,proto3" json:"info,omitempty"`                                                                           // Info contains optional information about publisher. Usually it is set only if publication goes from the client side.
	Offset        uint64                 `protobuf:"varint,6,opt,name=offset,proto3" json:"offset,omitempty"`                                                                      // Offset is a stream offset of the publication. Epoch is given in SubscribeResult.
	Tags          map[string]string      `protobuf:"bytes,7,rep,name=tags,proto3" json:"tags,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"` // Optional tags associated with publication.
	Delta         bool                   `protobuf:"varint,8,opt,name=delta,proto3" json:"delta,omitempty"`                                                                        // When set indicates that data in Publication is a delta from previous data.
	Time          int64                  `protobuf:"varint,9,opt,name=time,proto3" json:"time,omitempty"`                                                                          // Optional time of publication as Unix timestamp milliseconds.
	Channel       string                 `protobuf:"bytes,10,opt,name=channel,proto3" json:"channel,omitempty"`                                                                    // Optional channel name if Publication relates to wildcard subscription.
	Key           string                 `protobuf:"bytes,11,opt,name=key,proto3" json:"key,omitempty"`                                                                            // Optional key associated with publication.
	Removed       bool                   `protobuf:"varint,12,opt,name=removed,proto3" json:"removed,omitempty"`                                                                   // When set indicates that this publication is a removal of a previously published item.
	Score         int64                  `protobuf:"zigzag64,13,opt,name=score,proto3" json:"score,omitempty"`                                                                     // Represents score to order.
	Epoch         string                 `protobuf:"bytes,14,opt,name=epoch,proto3" json:"epoch,omitempty"`                                                                        // Optional epoch.
	PrevData      Raw                    `protobuf:"bytes,15,opt,name=prev_data,json=prevData,proto3" json:"prev_data,omitempty"`                                                  // Previous data for delta computation in broker fan-out.
	Version       uint64                 `protobuf:"varint,16,opt,name=version,proto3" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Join struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Info          *ClientInfo            `protobuf:"bytes,1,opt,name=info,proto3" json:"info,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Leave struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Info          *ClientInfo            `protobuf:"bytes,1,opt,name=info,proto3" json:"info,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Unsubscribe struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          uint32                 `protobuf:"varint,2,opt,name=code,proto3" json:"code,omitempty"`
	Reason        string                 `protobuf:"bytes,3,opt,name=reason,proto3" json:"reason,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Subscribe struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Recoverable   bool                   `protobuf:"varint,1,opt,name=recoverable,proto3" json:"recoverable,omitempty"`
	Epoch         string                 `protobuf:"bytes,4,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Offset        uint64                 `protobuf:"varint,5,opt,name=offset,proto3" json:"offset,omitempty"`
	Positioned    bool                   `protobuf:"varint,6,opt,name=positioned,proto3" json:"positioned,omitempty"`
	Data          Raw                    `protobuf:"bytes,7,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Message struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          Raw                    `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Connect struct {
	state         protoimpl.MessageState      `protogen:"open.v1"`
	Client        string                      `protobuf:"bytes,1,opt,name=client,proto3" json:"client,omitempty"`
	Version       string                      `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Data          Raw                         `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	Subs          map[string]*SubscribeResult `protobuf:"bytes,4,rep,name=subs,proto3" json:"subs,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	Expires       bool                        `protobuf:"varint,5,opt,name=expires,proto3" json:"expires,omitempty"`
	Ttl           uint32                      `protobuf:"varint,6,opt,name=ttl,proto3" json:"ttl,omitempty"`
	Ping          uint32                      `protobuf:"varint,7,opt,name=ping,proto3" json:"ping,omitempty"`
	Pong          bool                        `protobuf:"varint,8,opt,name=pong,proto3" json:"pong,omitempty"`
	Session       string                      `protobuf:"bytes,9,opt,name=session,proto3" json:"session,omitempty"`
	Node          string                      `protobuf:"bytes,10,opt,name=node,proto3" json:"node,omitempty"`
	Time          int64                       `protobuf:"varint,11,opt,name=time,proto3" json:"time,omitempty"` // Server time as Unix timestamp in milliseconds (not sent by default).
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Disconnect struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          uint32                 `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Reason        string                 `protobuf:"bytes,2,opt,name=reason,proto3" json:"reason,omitempty"`
	Reconnect     bool                   `protobuf:"varint,3,opt,name=reconnect,proto3" json:"reconnect,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type Refresh struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Expires       bool                   `protobuf:"varint,1,opt,name=expires,proto3" json:"expires,omitempty"`
	Ttl           uint32                 `protobuf:"varint,2,opt,name=ttl,proto3" json:"ttl,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type ConnectRequest struct {
	state   protoimpl.MessageState       `protogen:"open.v1"`
	Token   string                       `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
	Data    Raw                          `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Subs    map[string]*SubscribeRequest `protobuf:"bytes,3,rep,name=subs,proto3" json:"subs,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	Name    string                       `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	Version string                       `protobuf:"bytes,5,opt,name=version,proto3" json:"version,omitempty"`
	Headers map[string]string            `protobuf:"bytes,6,rep,name=headers,proto3" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	// Connection level features this client supports, as a bitmask. The server
	// replies with the subset it enabled in ConnectResult.flag.
	Flag int64 `protobuf:"varint,7,opt,name=flag,proto3" json:"flag,omitempty"`
	// Application context this connection belongs to - which view, screen or
	// client kind it is. Connections sharing a profile see traffic of a similar
	// shape.
	//
	// Untrusted: the server may override it, and features built on it must treat
	// it as a hint rather than an assertion.
	Profile string `protobuf:"bytes,9,opt,name=profile,proto3" json:"profile,omitempty"`
	// Id of the compression dictionary this client already holds, so the server
	// can name it instead of sending it again.
	//
	// The server answers with a Dictionary carrying only an id when it recognises
	// this one, and with the full content otherwise - so an unknown id is a cache
	// miss rather than an error.
	//
	// An id is a hash of the dictionary content, so this id and the server's copy
	// are byte identical by construction: a dictionary that changes gets a new id
	// automatically.
	//
	// Deliberately one, not a list. A client caches the last dictionary it was
	// given, which is the one it needs when reconnecting to the same profile. The
	// cost is one extra transfer per hop during a rolling deploy where nodes
	// disagree, which is bounded and small.
	Dict          string `protobuf:"bytes,8,opt,name=dict,proto3" json:"dict,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type ConnectResult struct {
	state   protoimpl.MessageState      `protogen:"open.v1"`
	Client  string                      `protobuf:"bytes,1,opt,name=client,proto3" json:"client,omitempty"`
	Version string                      `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Expires bool                        `protobuf:"varint,3,opt,name=expires,proto3" json:"expires,omitempty"`
	Ttl     uint32                      `protobuf:"varint,4,opt,name=ttl,proto3" json:"ttl,omitempty"`
	Data    Raw                         `protobuf:"bytes,5,opt,name=data,proto3" json:"data,omitempty"`
	Subs    map[string]*SubscribeResult `protobuf:"bytes,6,rep,name=subs,proto3" json:"subs,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	Ping    uint32                      `protobuf:"varint,7,opt,name=ping,proto3" json:"ping,omitempty"`
	Pong    bool                        `protobuf:"varint,8,opt,name=pong,proto3" json:"pong,omitempty"`
	Session string                      `protobuf:"bytes,9,opt,name=session,proto3" json:"session,omitempty"`
	Node    string                      `protobuf:"bytes,10,opt,name=node,proto3" json:"node,omitempty"`
	Time    int64                       `protobuf:"varint,11,opt,name=time,proto3" json:"time,omitempty"` // Server time as Unix timestamp in milliseconds (not sent by default).
	// Connection level features the server actually enabled, using the same bits
	// as ConnectRequest.flag. A client can not assume a feature it advertised was
	// accepted - the server may have it disabled, or may decline per connection -
	// so this is how it finds out.
	Flag int64 `protobuf:"varint,12,opt,name=flag,proto3" json:"flag,omitempty"`
	// Compression dictionary for this connection, when the server enabled
	// dictionary compression.
	//
	// It travels in the connect reply rather than a push so there is no ordering
	// rule to get wrong: this frame is raw, and every frame after it is
	// compressed. It carries only an id when the client advertised that same id in
	// ConnectRequest.dict, since the client already holds the bytes.
	Dict          *Dictionary `protobuf:"bytes,13,opt,name=dict,proto3" json:"dict,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type RefreshRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Token         string                 `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type RefreshResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Client        string                 `protobuf:"bytes,1,opt,name=client,proto3" json:"client,omitempty"`
	Version       string                 `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Expires       bool                   `protobuf:"varint,3,opt,name=expires,proto3" json:"expires,omitempty"`
	Ttl           uint32                 `protobuf:"varint,4,opt,name=ttl,proto3" json:"ttl,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type SubscribeRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	Token         string                 `protobuf:"bytes,2,opt,name=token,proto3" json:"token,omitempty"`
	Recover       bool                   `protobuf:"varint,3,opt,name=recover,proto3" json:"recover,omitempty"`
	Epoch         string                 `protobuf:"bytes,6,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Offset        uint64                 `protobuf:"varint,7,opt,name=offset,proto3" json:"offset,omitempty"`
	Data          Raw                    `protobuf:"bytes,8,opt,name=data,proto3" json:"data,omitempty"`
	Positioned    bool                   `protobuf:"varint,9,opt,name=positioned,proto3" json:"positioned,omitempty"`
	Recoverable   bool                   `protobuf:"varint,10,opt,name=recoverable,proto3" json:"recoverable,omitempty"`
	JoinLeave     bool                   `protobuf:"varint,11,opt,name=join_leave,json=joinLeave,proto3" json:"join_leave,omitempty"`
	Delta         string                 `protobuf:"bytes,12,opt,name=delta,proto3" json:"delta,omitempty"`
	Tf            *FilterNode            `protobuf:"bytes,13,opt,name=tf,proto3" json:"tf,omitempty"`      // Optional server side filter based on publication tags .
	Flag          int64                  `protobuf:"varint,14,opt,name=flag,proto3" json:"flag,omitempty"` // Enable subscription level features.
	Type          int32                  `protobuf:"varint,15,opt,name=type,proto3" json:"type,omitempty"`
	Phase         int32                  `protobuf:"varint,16,opt,name=phase,proto3" json:"phase,omitempty"`
	Cursor        string                 `protobuf:"bytes,17,opt,name=cursor,proto3" json:"cursor,omitempty"`
	Limit         int32                  `protobuf:"varint,18,opt,name=limit,proto3" json:"limit,omitempty"`
	Asc           bool                   `protobuf:"varint,19,opt,name=asc,proto3" json:"asc,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type SubscribeResult struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	Expires         bool                   `protobuf:"varint,1,opt,name=expires,proto3" json:"expires,omitempty"`
	Ttl             uint32                 `protobuf:"varint,2,opt,name=ttl,proto3" json:"ttl,omitempty"`
	Recoverable     bool                   `protobuf:"varint,3,opt,name=recoverable,proto3" json:"recoverable,omitempty"`
	Epoch           string                 `protobuf:"bytes,6,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Publications    []*Publication         `protobuf:"bytes,7,rep,name=publications,proto3" json:"publications,omitempty"`
	Recovered       bool                   `protobuf:"varint,8,opt,name=recovered,proto3" json:"recovered,omitempty"`
	Offset          uint64                 `protobuf:"varint,9,opt,name=offset,proto3" json:"offset,omitempty"`
	Positioned      bool                   `protobuf:"varint,10,opt,name=positioned,proto3" json:"positioned,omitempty"`
	Data            Raw                    `protobuf:"bytes,11,opt,name=data,proto3" json:"data,omitempty"`
	WasRecovering   bool                   `protobuf:"varint,12,opt,name=was_recovering,json=wasRecovering,proto3" json:"was_recovering,omitempty"`
	Delta           bool                   `protobuf:"varint,13,opt,name=delta,proto3" json:"delta,omitempty"`
	Id              int64                  `protobuf:"varint,14,opt,name=id,proto3" json:"id,omitempty"`                                                  // Optional numeric channel ID to avoid sending string channel in the following Pushes (bandwidth optimization).
	Type            int32                  `protobuf:"varint,15,opt,name=type,proto3" json:"type,omitempty"`                                              // Server must echo back type.
	Phase           int32                  `protobuf:"varint,16,opt,name=phase,proto3" json:"phase,omitempty"`                                            // The result phase of the operation (LIVE = 0, STREAM = 1, STATE = 2)
	Cursor          string                 `protobuf:"bytes,17,opt,name=cursor,proto3" json:"cursor,omitempty"`                                           // Next page cursor (empty = last page)
	State           []*Publication         `protobuf:"bytes,18,rep,name=state,proto3" json:"state,omitempty"`                                             // Channel state entries.
	PublishDebounce uint32                 `protobuf:"varint,19,opt,name=publish_debounce,json=publishDebounce,proto3" json:"publish_debounce,omitempty"` // in ms, if >0 SDK should debounce publications to this channel. First one should not be debounced.
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

type KeyedItem struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Key           string                 `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Version       uint64                 `protobuf:"varint,2,opt,name=version,proto3" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type TrackBatch struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Signature     string                 `protobuf:"bytes,1,opt,name=signature,proto3" json:"signature,omitempty"` // HMAC over (iat, expiry, user, channel, keys)
	Items         []*KeyedItem           `protobuf:"bytes,2,rep,name=items,proto3" json:"items,omitempty"`         // FULL key set the signature was computed over;
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type SubRefreshRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	Token         string                 `protobuf:"bytes,2,opt,name=token,proto3" json:"token,omitempty"`
	Type          int32                  `protobuf:"varint,3,opt,name=type,proto3" json:"type,omitempty"`      // 0=sub_refresh, 1=track, 2=untrack
	Track         []*TrackBatch          `protobuf:"bytes,4,rep,name=track,proto3" json:"track,omitempty"`     // for type=1
	Untrack       []string               `protobuf:"bytes,5,rep,name=untrack,proto3" json:"untrack,omitempty"` // for type=2
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type SubRefreshResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Expires       bool                   `protobuf:"varint,1,opt,name=expires,proto3" json:"expires,omitempty"`
	Ttl           uint32                 `protobuf:"varint,2,opt,name=ttl,proto3" json:"ttl,omitempty"` // for type=0: token TTL. for type=1: MIN TTL across all batches.
	Items         []*Publication         `protobuf:"bytes,3,rep,name=items,proto3" json:"items,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type UnsubscribeRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type UnsubscribeResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PublishRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	Data          Raw                    `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Type          int32                  `protobuf:"varint,3,opt,name=type,proto3" json:"type,omitempty"`       // 0 = regular (default), 1 = map
	Key           string                 `protobuf:"bytes,4,opt,name=key,proto3" json:"key,omitempty"`          // Map publish: key to publish/remove
	Removed       bool                   `protobuf:"varint,5,opt,name=removed,proto3" json:"removed,omitempty"` // Map publish: true = remove this key
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PublishResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PresenceRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PresenceResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Presence      map[string]*ClientInfo `protobuf:"bytes,1,rep,name=presence,proto3" json:"presence" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PresenceStatsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PresenceStatsResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	NumClients    uint32                 `protobuf:"varint,1,opt,name=num_clients,json=numClients,proto3" json:"num_clients"`
	NumUsers      uint32                 `protobuf:"varint,2,opt,name=num_users,json=numUsers,proto3" json:"num_users"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type StreamPosition struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Offset        uint64                 `protobuf:"varint,1,opt,name=offset,proto3" json:"offset,omitempty"`
	Epoch         string                 `protobuf:"bytes,2,opt,name=epoch,proto3" json:"epoch,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type HistoryRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Channel       string                 `protobuf:"bytes,1,opt,name=channel,proto3" json:"channel,omitempty"`
	Limit         int32                  `protobuf:"varint,7,opt,name=limit,proto3" json:"limit,omitempty"`
	Since         *StreamPosition        `protobuf:"bytes,8,opt,name=since,proto3" json:"since,omitempty"`
	Reverse       bool                   `protobuf:"varint,9,opt,name=reverse,proto3" json:"reverse,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type HistoryResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Publications  []*Publication         `protobuf:"bytes,1,rep,name=publications,proto3" json:"publications"`
	Epoch         string                 `protobuf:"bytes,2,opt,name=epoch,proto3" json:"epoch"`
	Offset        uint64                 `protobuf:"varint,3,opt,name=offset,proto3" json:"offset"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PingRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type PingResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type RPCRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          Raw                    `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Method        string                 `protobuf:"bytes,2,opt,name=method,proto3" json:"method,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type RPCResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          Raw                    `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type SendRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          Raw                    `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type FilterNode struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Operation type for this node:
	// - "" (empty string) → leaf node (comparison)
	// - "and" → logical AND of child nodes
	// - "or"  → logical OR of child nodes
	// - "not" → logical NOT of a single child node
	Op string `protobuf:"bytes,1,opt,name=op,proto3" json:"op,omitempty"`
	// Key for comparison (only valid for leaf nodes).
	Key string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	// Comparison operator for leaf nodes.
	// Only meaningful if op == "".
	// Supported values:
	//
	//	"eq"   → equal
	//	"neq"  → not equal
	//	"in"   → value is in vals
	//	"nin"  → value is not in vals
	//	"ex"   → key exists in tags
	//	"nex"  → key does not exist
	//	"sw"   → string starts with val
	//	"ew"   → string ends with val
	//	"ct"   → string contains val
	//	"lt"   → numeric less than val
	//	"lte"  → numeric less than or equal val
	//	"gt"   → numeric greater than val
	//	"gte"  → numeric greater than or equal val
	Cmp string `protobuf:"bytes,3,opt,name=cmp,proto3" json:"cmp,omitempty"`
	// Single value used in most comparisons (e.g. "eq").
	Val string `protobuf:"bytes,4,opt,name=val,proto3" json:"val,omitempty"`
	// Multiple values used for set comparisons ("in", "nin").
	Vals []string `protobuf:"bytes,5,rep,name=vals,proto3" json:"vals,omitempty"`
	// Child nodes.
	// Used for logical operations: "and", "or", "not".
	Nodes         []*FilterNode `protobuf:"bytes,6,rep,name=nodes,proto3" json:"nodes,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}
