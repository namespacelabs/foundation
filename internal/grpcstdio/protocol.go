// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Package grpcstdio implements a custom (but simple) multiplexing protocol over
// a bidirectional guaranteed delivery stream. It is used for `fn` to
// communicate with containerized processes which are not given any other kind
// of network access. A pair of {stdin, stdout} is used as the bidirectional
// stream. A custom protocol is introduced that is carefully designed to remove
// the need to handshake. This is important to manage latency; previous
// iterations had an explicit handshake, and together with scheduling delays w/
// having a container start, we could see upwards of seconds in high contention
// systems (not just low-end, but also typically in CI). Checksums are used to
// ensure that the frames were not intermixed with other data.

package grpcstdio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/versions"
)

// The protocol uses variable-sized frames, emitted in either direction. Each
// frame follows the following structure:
//
// +---------+--------------+--------------+----------------+---------------+----------------+
// | Op (4b) | Length (28b) | Stream (16b) | Reserved (16b) | Payload (...) | Checksum (64b) |
// +---------+--------------+--------------+----------------+---------------+----------------+
//
// Each word is encoded in big-endian.

const (
	opHello           op = 0x1 // A session must always begin with an hello; the frame payload is a serialized Hello message.
	opDial            op = 0x2 // A new stream must always be explicitly established. The caller selects the stream id.
	opSendToServer    op = 0x3 // The caller (previous user of opNewStream) pushes bytes to the specified stream id.
	opSendToClient    op = 0x4 // The peer pushes data should be received in the specified stream id (i.e. a previously created stream).
	opCloseClientSide op = 0x5 // The caller tells the peer that it will no longer use the stream specified.
	opCloseServerSide op = 0x6 // The receiver tells the stream creator that it will no longer use the stream specified.
	opServerError     op = 0x7 // The receiver tells the caller that dialing, or sending data to the specified stream failed.
	opClientError     op = 0x8 // The receiver tells the caller that dialing, or sending data to the specified stream failed.

	dirServer direction = 1 // Accept'd
	dirClient direction = 2 // Dial'd
)

type op int
type direction int

type msg struct {
	op_length       uint32
	stream_reserved uint32
	checksum        uint64
	payload         []byte
}

type Session struct {
	r       *bufferedPipeReader
	rwriter *bufferedPipeWriter // `r`'s pair.
	w       *bufferedPipeWriter
	wreader *bufferedPipeReader // `w`'s pair.

	debugf        func(string, ...interface{})
	onCloseStream func(*Stream)

	mu           sync.Mutex
	cond         *sync.Cond
	sessionAlloc uint32
	serr         error
	ourStreams   map[uint32]*Stream
	peerStreams  map[uint32]*Stream
	pending      []*DialedStream
}

type DialedStream struct {
	*Stream
	Args *DialArgs
}

type Stream struct {
	parent    *Session
	id        uint32
	direction direction

	pw *bufferedPipeWriter
	pr *bufferedPipeReader
}

type NewSessionOpt func(*Session)

func NewSession(ctx context.Context, r io.Reader, w io.Writer, opts ...NewSessionOpt) (*Session, error) {
	w_pr, w_pw := newBufferedPipe()
	r_pr, r_pw := newBufferedPipe()

	sess := &Session{
		r:       r_pr,
		rwriter: r_pw,
		// A buffered writer is used, instead of the underlying writer, in order to not
		// depend on the ability for that writer to perform atomic concurrent writes.
		w:            w_pw,
		wreader:      w_pr,
		sessionAlloc: 1,
		ourStreams:   map[uint32]*Stream{},
		peerStreams:  map[uint32]*Stream{},
		debugf:       func(s string, a ...interface{}) {},
	}

	for _, opt := range opts {
		opt(sess)
	}

	sess.cond = sync.NewCond(&sess.mu)

	hello := &HelloArgs{
		FnApiVersion:   versions.APIVersion,
		ToolApiVersion: versions.ToolAPIVersion,
	}
	helloBytes, err := proto.Marshal(hello)
	if err != nil {
		return nil, err
	}

	if err := sess.sendRaw(makeMsg(opHello, 0, helloBytes)); err != nil {
		return nil, err
	}

	go func() {
		_, err := io.Copy(w, w_pr)
		sess.debugf("leaving w->w_pr goroutine: %v", err)
		if err == nil {
			err = io.EOF
		}
		_ = w_pr.closeWithError(err)
	}()

	go func() {
		_, err := io.Copy(r_pw, r)
		sess.debugf("leaving r->r_pw goroutine: %v", err)
		if err == nil {
			err = io.EOF
		}
		_ = r_pw.closeWithError(err)
	}()

	go func() {
		<-ctx.Done()
		sess.debugf("context cancelled")
		// On cancelation, close the reader with a cancelation error.
		_ = r_pw.closeWithError(ctx.Err())
	}()

	go sess.loop()

	return sess, nil
}

func WithDebug(f func(string, ...interface{})) NewSessionOpt {
	return func(s *Session) {
		s.debugf = f
	}
}

func WithCloseNotifier(f func(*Stream)) NewSessionOpt {
	return func(s *Session) {
		s.onCloseStream = f
	}
}

func (m msg) op() op {
	return op((m.op_length & 0xf000_0000) >> 28)
}

func (m msg) length() uint32 {
	return m.op_length & 0x0fff_ffff
}

func (m msg) streamID() uint32 {
	return (m.stream_reserved >> 16) & 0xffff
}

func (m msg) serializePayloadTo(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, m.op_length); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.stream_reserved); err != nil {
		return err
	}
	if _, err := w.Write(m.payload); err != nil {
		return err
	}
	return nil
}

func (m msg) serializeTo(w io.Writer) error {
	if err := m.serializePayloadTo(w); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.calculateChecksum()); err != nil {
		return err
	}
	return nil
}

func (m msg) calculateChecksum() uint64 {
	h := xxhash.New()

	if err := m.serializePayloadTo(h); err != nil {
		panic("failed to write hash")
	}

	return h.Sum64()
}

func (s *Session) Listener() net.Listener {
	return sessionListener{s}
}

func (s *Session) Dial(dial *DialArgs) (*Stream, error) {
	dialBytes, err := proto.Marshal(dial)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.sessionAlloc
	s.sessionAlloc++

	if s.sessionAlloc > 0xffff {
		panic("session allocation wrapped around")
	}

	stream, err := s.newStream(dirClient, id)
	if err != nil {
		return nil, err
	}

	if err := s.sendRaw(makeMsg(opDial, id, dialBytes)); err != nil {
		return nil, err
	}

	s.ourStreams[id] = stream
	return stream, nil
}

func (s *Session) readmsg() (msg, error) {
	var msg msg
	var err error
	msg.op_length, err = s.readword()
	if err != nil {
		return msg, err
	}

	msg.stream_reserved, err = s.readword()
	if err != nil {
		return msg, err
	}

	msg.payload, err = s.mustread(msg.length())
	if err != nil {
		return msg, err
	}

	msg.checksum, err = s.readlongword()
	if err != nil {
		return msg, err
	}

	return msg, nil
}

func (s *Session) mustread(req uint32) ([]byte, error) {
	block := make([]byte, req)
	index := uint32(0)

	for index < req {
		n, err := s.r.Read(block[index:])
		if err != nil {
			return nil, err
		}
		if n < 0 {
			panic("negative read")
		}
		index += uint32(n)
	}

	return block, nil
}

func (s *Session) readword() (uint32, error) {
	word, err := s.mustread(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(word), nil
}

func (s *Session) readlongword() (uint64, error) {
	word, err := s.mustread(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(word), nil
}

func (s *Session) loop() {
	for {
		msg, err := s.readmsg()
		if err != nil {
			s.quit(err)
			break
		}

		if msg.checksum != msg.calculateChecksum() {
			s.quit(errors.New("bad checksum"))
			break
		}

		s.handle(msg)
	}
}

func (s *Session) handle(msg msg) {
	s.debugf("handle op=%s sid=%d [%x %x]", msg.op(), msg.streamID(), msg.op_length, msg.stream_reserved)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.op() {
	case opHello:

	case opDial:
		dial := &DialArgs{}
		if err := proto.Unmarshal(msg.payload, dial); err != nil {
			s.send(serverError(msg.streamID(), err))
		} else {
			if _, has := s.peerStreams[msg.streamID()]; has {
				s.send(serverError(msg.streamID(), status.Error(codes.AlreadyExists, "stream already exists")))
			} else {
				newStream, err := s.newStream(dirServer, msg.streamID())
				if err != nil {
					s.send(serverError(msg.streamID(), err))
				} else {
					s.peerStreams[msg.streamID()] = newStream
					s.pending = append(s.pending, &DialedStream{newStream, dial})
					s.cond.Signal() // Wake up one Accept().
				}
			}
		}

	case opSendToServer:
		if stream, has := s.peerStreams[msg.streamID()]; has {
			if err := stream.received(msg.payload); err != nil {
				stream.failed(err)
			}
		} else {
			s.send(serverError(msg.streamID(), status.Error(codes.NotFound, "no such stream")))
		}

	case opSendToClient:
		if stream, has := s.ourStreams[msg.streamID()]; has {
			if err := stream.received(msg.payload); err != nil {
				stream.failed(err)
			}
		} else {
			s.send(clientError(msg.streamID(), status.Error(codes.NotFound, "no such stream")))
		}

	case opCloseClientSide:
		if stream, has := s.peerStreams[msg.streamID()]; has {
			s.finStream(stream, io.EOF)
		}

	case opCloseServerSide:
		if stream, has := s.ourStreams[msg.streamID()]; has {
			s.finStream(stream, io.EOF)
		}

	case opServerError:
		if stream, has := s.ourStreams[msg.streamID()]; has {
			st := &spb.Status{}
			if err := proto.Unmarshal(msg.payload, st); err != nil {
				st = status.New(codes.Internal, "unknown error").Proto()
			}

			s.debugf("server error for stream %x: %v", msg.streamID(), st)
			s.finStream(stream, status.FromProto(st).Err())
		}

	case opClientError:
		if stream, has := s.peerStreams[msg.streamID()]; has {
			st := &spb.Status{}
			if err := proto.Unmarshal(msg.payload, st); err != nil {
				st = status.New(codes.Internal, "unknown error").Proto()
			}

			s.debugf("client error for stream %x: %v", msg.streamID(), st)
			s.finStream(stream, status.FromProto(st).Err())
		}
	}
}

func (s *Session) newStream(dir direction, id uint32) (*Stream, error) {
	pr, pw := newBufferedPipe()

	stream := &Stream{parent: s, id: id, direction: dir, pr: pr, pw: pw}

	return stream, nil
}

func (s *Session) send(m msg) {
	if err := s.sendRaw(m); err != nil {
		s.quit(err)
	}
}

func (s *Session) sendRaw(m msg) error {
	var buf bytes.Buffer // XXX re-use buffers.

	if err := m.serializeTo(&buf); err != nil {
		return err
	} else {
		s.debugf("sendRaw op=%s %d bytes [%x %x]", m.op(), len(m.payload), m.op_length, m.stream_reserved)
		// Doesn't block to write to the actual destination, as `w` is a buffered pipe.
		if _, err := buf.WriteTo(s.w); err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) quit(err error) {
	s.debugf("quit: %v", err)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.serr = err
	s.cond.Broadcast()

	for _, s := range s.ourStreams {
		s.closePipes(err)
	}

	for _, s := range s.peerStreams {
		s.closePipes(err)
	}

	s.ourStreams = nil
	s.peerStreams = nil

	_ = s.r.closeWithError(err)
	_ = s.rwriter.closeWithError(err)
	_ = s.wreader.closeWithError(err)
	_ = s.w.closeWithError(err)
}

func (s *Session) writeToStream(dir direction, id uint32, p []byte) (int, error) {
	if len(p) > 0x0fff_ffff {
		return 0, errors.New("payload is too large")
	}

	op := opSendToServer
	if dir == dirServer {
		op = opSendToClient
	}

	err := s.sendRaw(makeMsg(op, id, p))
	return len(p), err
}

func (s *Session) Accept() (*DialedStream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.pending) == 0 {
		s.cond.Wait()

		if s.serr != nil {
			return nil, s.serr
		}
	}

	conn := s.pending[0]

	s.debugf("got connection %s %x %v", conn.Stream.direction, conn.Stream.id, conn.Args)

	s.pending = s.pending[1:]
	return conn, nil
}

func (s *Session) Shutdown() {
	s.debugf("close listener")
	s.quit(errors.New("listener closed"))
}

func (s *Session) closeStream(stream *Stream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeStreamUnsafe(stream)
}

func (s *Session) closeStreamUnsafe(stream *Stream) {
	if stream.direction == dirClient {
		if _, ok := s.ourStreams[stream.id]; !ok {
			return
		}

		s.debugf("close stream %s %x", stream.direction, stream.id)
		s.send(makeMsg(opCloseClientSide, stream.id, nil))
	} else {
		if _, ok := s.peerStreams[stream.id]; !ok {
			return
		}

		s.debugf("close stream %s %x", stream.direction, stream.id)
		s.send(makeMsg(opCloseServerSide, stream.id, nil))
	}

	s.finStream(stream, nil)
}

func (s *Session) finStream(stream *Stream, err error) {
	if err != nil {
		stream.closePipes(err)
	}

	if stream.direction == dirClient {
		delete(s.ourStreams, stream.id)
	} else {
		delete(s.peerStreams, stream.id)
	}

	if s.onCloseStream != nil {
		s.onCloseStream(stream)
	}
}

func (s *Stream) received(p []byte) error {
	_, err := s.pw.Write(p)
	return err
}

func (s *Stream) failed(err error) {
	s.parent.debugf("stream failed %s %x: %v", s.direction, s.id, err)
	s.closePipes(err)
	s.parent.closeStreamUnsafe(s)
}

func (s *Stream) closePipes(err error) {
	_ = s.pr.closeWithError(err)
	_ = s.pw.closeWithError(err)
}

func (s *Stream) Read(p []byte) (int, error) {
	return s.pr.Read(p)
}

func (s *Stream) Write(p []byte) (int, error) {
	return s.parent.writeToStream(s.direction, s.id, p)
}

func (s *Stream) Close() error {
	s.parent.debugf("stream.close %s %x", s.direction, s.id)
	s.closePipes(errors.New("closed"))
	s.parent.closeStream(s)
	return nil
}

func (s *Stream) LocalAddr() net.Addr {
	return stdioAddr{network: "stdio", str: "local"}
}

func (s *Stream) RemoteAddr() net.Addr {
	return stdioAddr{network: "stdio", str: "remote"}
}

func (s *Stream) SetDeadline(t time.Time) error {
	// Not supported.
	return nil
}
func (s *Stream) SetReadDeadline(t time.Time) error {
	// Not supported.
	return nil
}
func (s *Stream) SetWriteDeadline(t time.Time) error {
	// Not supported.
	return nil
}

func clientError(streamID uint32, err error) msg {
	return makeError(opClientError, streamID, err)
}

func serverError(streamID uint32, err error) msg {
	return makeError(opServerError, streamID, err)
}

func makeError(op op, streamID uint32, err error) msg {
	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Internal, err.Error())
	}

	var payload []byte
	var marshalErr error

	payload, marshalErr = proto.Marshal(s.Proto())
	if marshalErr != nil {
		payload = nil
	}

	return makeMsg(op, streamID, payload)
}

func makeMsg(op op, streamID uint32, payload []byte) msg {
	// XXX check payload size
	var msg msg
	msg.op_length = ((uint32(op) << 28) & 0xf000_0000) | uint32(len(payload))
	msg.stream_reserved = (streamID << 16) & 0xffff_0000
	msg.payload = payload
	return msg
}

type sessionListener struct {
	parent *Session
}

func (lis sessionListener) Accept() (net.Conn, error) {
	return lis.parent.Accept()
}

func (lis sessionListener) Close() error {
	lis.parent.Shutdown()
	return nil
}

func (lis sessionListener) Addr() net.Addr {
	return stdioAddr{"stdio", "listener"}
}

func (op op) String() string {
	switch op {
	case opHello:
		return "hello"
	case opDial:
		return "dial"
	case opSendToServer:
		return "send-to-server"
	case opSendToClient:
		return "send-to-client"
	case opCloseClientSide:
		return "close-client"
	case opCloseServerSide:
		return "close-server"
	case opServerError:
		return "server-error"
	case opClientError:
		return "client-error"
	default:
		return "unknown"
	}
}

func (dir direction) String() string {
	switch dir {
	case dirClient:
		return "client"
	case dirServer:
		return "server"
	default:
		return "unknown"
	}
}
