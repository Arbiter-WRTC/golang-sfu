package Client

import (
	"log"
	"sync"
	"github.com/gorilla/websocket"
	"arbiter-go-sfu/pkg/producer" 
	"arbiter-go-sfu/pkg/arbiterTypes"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	sync.RWMutex
	clientId string
	rtcConfig []webrtc.ICEServer
	socket *websocket.Conn
	eventEmitter  map[string]chan string
	producer *Producer.Producer
}

type HandshakePayload struct {
	Type string `json:"type"`
	Sender string `json:"sender"`
	Description SessionDescription `json:"description"`
	Candidate IceCandidate `json:"candidate"`
}

type SessionDescription struct {
	Type string `json:"type"`
	Sdp string `json:"sdp"`
}

type IceCandidate struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex int    `json:"sdpMLineIndex"`
}

func NewClient(clientId string, rtcConfig []webrtc.ICEServer, socket *websocket.Conn, eventEmitter map[string]chan string) *Client {
	client := &Client{
		clientId: clientId,
		rtcConfig: rtcConfig,
		socket: socket,
		eventEmitter: eventEmitter,
		producer: Producer.NewProducer(clientId, rtcConfig, socket, eventEmitter),
	}
	
	return client
}

func (client *Client) ProducerHandshake(data ArbiterTypes.HandshakePayload) {
	client.producer.Handshake(data)
}

func (client *Client) PrintId() {
	log.Println(client.clientId)
}
