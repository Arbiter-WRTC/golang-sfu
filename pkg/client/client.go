package Client

import (
	"log"
	"sync"
	"arbiter-go-sfu/pkg/producer" 
	"arbiter-go-sfu/pkg/consumer"
	"arbiter-go-sfu/pkg/arbiterTypes"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	sync.RWMutex
	clientId 			string
	sfuId 				string
	rtcConfig 		[]webrtc.ICEServer
	socket 				*ArbiterTypes.SafeConnection
	producerTrackChannel chan ArbiterTypes.ProducerTrackChannel
	featuresSharedChannel chan string
	producer 			*Producer.Producer
	consumers			map[string] *Consumer.Consumer
}

type HandshakePayload struct {
	Type 				string 						 `json:"type"`
	Sender 			string 						 `json:"sender"`
	Description SessionDescription `json:"description"`
	Candidate 	IceCandidate 			 `json:"candidate"`
}

type SessionDescription struct {
	Type string `json:"type"`
	Sdp  string `json:"sdp"`
}

type IceCandidate struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex int    `json:"sdpMLineIndex"`
}

func NewClient(clientId, sfuId string, rtcConfig []webrtc.ICEServer, socket *ArbiterTypes.SafeConnection, producerTrackChannel chan ArbiterTypes.ProducerTrackChannel, featuresSharedChannel chan string) *Client {
	client := &Client{
		clientId: clientId,
		sfuId: sfuId,
		rtcConfig: rtcConfig,
		socket: socket,
		producerTrackChannel: producerTrackChannel,
		featuresSharedChannel: featuresSharedChannel,
		producer: Producer.NewProducer(clientId, sfuId, rtcConfig, socket, producerTrackChannel, featuresSharedChannel),
		consumers: make(map[string] *Consumer.Consumer),
	}
	
	return client
}

func (client *Client) Id() string {
	return client.clientId
}

func (client *Client) Producer() *Producer.Producer {
	return client.producer
}

func (client *Client) ProducerHandshake(data ArbiterTypes.HandshakePayload) {
	client.producer.Handshake(data)
}

func (client *Client) ConsumerHandshake(data ArbiterTypes.HandshakePayload) {
	remotePeerId := data.RemotePeer
	consumer, ok := client.consumers[remotePeerId]

	if !ok {
		log.Println("error, consumer not found!")
	}

	consumer.Handshake(data)
}

func (client *Client) GetProducerTrack(kind string) *webrtc.TrackRemote {
	return client.producer.GetMediaTracks()[kind]
}

func (client *Client) AddConsumerTrack(remotePeerId string, track *webrtc.TrackRemote) {
	consumer, ok := client.consumers[remotePeerId]

	if !ok {
		consumer = client.CreateConsumer(remotePeerId)
	}

	consumer.AddTrack(track)
}

func (client *Client) FindConsumerById(remotePeerId string) *Consumer.Consumer {
	return client.consumers[remotePeerId]
}

func (client *Client) CreateConsumer(remotePeerId string) *Consumer.Consumer {
	consumer := Consumer.NewConsumer(
		client.clientId,
		client.sfuId,
		remotePeerId,
		client.rtcConfig,
		client.socket, 
	)

	client.consumers[remotePeerId] = consumer
	return client.consumers[remotePeerId]
}

func (client *Client) ShareFeatures() {

}

func (client *Client) SetFeatures() {
	
}

func (client *Client) GetFeatures() {
	
}

func (client *Client) PruneClient() {
	client.producer.CloseConnection()
	for _, consumer := range client.consumers {
		consumer.CloseConnection()
	}
}