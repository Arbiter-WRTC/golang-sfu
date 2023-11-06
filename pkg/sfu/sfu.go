package SFU

import ( 
	"fmt"
	"time"
	"sync"
	"github.com/gorilla/websocket" 
	"log"
	"encoding/json"
	"arbiter-go-sfu/pkg/client"
	"arbiter-go-sfu/pkg/arbiterTypes"
	"github.com/pion/webrtc/v3"
)

type SFU struct {
	sync.RWMutex
	clients 	    map[string]*Client.Client
	socket        *ArbiterTypes.SafeConnection
	producerTrackChannel chan ArbiterTypes.ProducerTrackChannel
	featuresSharedChannel chan string
	sfuId 		    string
	rtcConfig     []webrtc.ICEServer
}

type IdentifyMessage struct {
    Action string 				 `json:"action"`
    Data   IdentifyPayload `json:"data"`	
}

type IdentifyPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}


func NewSFU(socketUrl, sfuId string, rtcConfig []webrtc.ICEServer) *SFU {
	socket, err := connectWebSocket(socketUrl, sfuId)
	if err != nil {
		log.Fatal(err)
		// return
	}

	featuresSharedChannel := make(chan string)
	producerTrackChannel := make(chan ArbiterTypes.ProducerTrackChannel)
	
	sfu := &SFU{
		clients: make(map[string]*Client.Client),
		socket: socket,
		featuresSharedChannel: featuresSharedChannel,
		producerTrackChannel: producerTrackChannel,
		sfuId: sfuId,
		rtcConfig: rtcConfig,
	}
	
	// starts a goroutine to listen to events that will occur in producer structs that need to be acted on
	// by other clients
	go sfu.listenForEvents()
	// identify sends a signal to the websocket server that tells it who it is, so it can be stored in the db
	sfu.identify()
	// starts a goroutine that loops listening for signals from the websocket server
	go sfu.listenOnSocket()
	return sfu
}

func connectWebSocket(socketUrl string, sfuId string) (*ArbiterTypes.SafeConnection, error) {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(socketUrl, nil)
	if err != nil {
			log.Fatal(err)
	}

	log.Println("Connected to WebSocket server:", socketUrl)
	
	return &ArbiterTypes.SafeConnection{Conn: conn,}, nil
}

func (sfu *SFU) identify() {
	payload := IdentifyMessage {
							Action: "identify",
							Data: IdentifyPayload {
								ID:   sfu.sfuId,
								Type: "sfu",
							},
    				}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to serialize JSON payload: %v\n", err)
		return
	}
	sfu.socket.Mu.Lock()
	defer sfu.socket.Mu.Unlock()
	err = sfu.socket.Conn.WriteMessage(websocket.TextMessage, payloadJSON)
	if err != nil {
		log.Printf("Failed to send payload over WebSocket: %v\n", err)
		return
	}
}

func (sfu *SFU) listenOnSocket() {
	pongWait := 60 * time.Second

	sfu.socket.Conn.SetReadDeadline(time.Now().Add(pongWait))
	sfu.socket.Conn.SetPongHandler(func(string) error { sfu.socket.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := sfu.socket.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		// Start a goroutine to process messages from the channel
		go sfu.handleMessage(message)
	}
}

func (sfu *SFU) listenForEvents() {
	for {
		select {
		case msg := <-sfu.featuresSharedChannel:
				fmt.Println("Feature shared event:", msg)
		case data := <-sfu.producerTrackChannel:
			log.Println("incoming producer track")
			sfu.handleProducerTrack(data)
		}
	}
}

func (sfu *SFU) handleMessage(data []byte) {
	var parsedData ArbiterTypes.HandshakePayload
	// check if its a sdp or an ice candidate
	// if it's an ice candidate, we need to marshal it better
	if err := json.Unmarshal(data, &parsedData); err != nil {
		log.Printf("Failed to parse JSON payload: %v\n", err)
		return
	}

	if parsedData.Type == "producer" {
		sfu.handleProducerHandshake(parsedData)
	} else if parsedData.Type == "consumer" {
		sfu.handleConsumerHandshake(parsedData)
	}
}

func (sfu *SFU) handleProducerTrack(data ArbiterTypes.ProducerTrackChannel) {
	// Id, Track
	for _, client := range sfu.clients {
		clientId := client.Id()
		if clientId == data.Id {
			sfu.consumerCatchup(clientId)
			continue
		}
		log.Println("attempting to add a track of type", data.Track.Kind())
		log.Println("the track is from", clientId)
		log.Println("the new producer is", data.Id)
		client.AddConsumerTrack(clientId, data.Track)
	}
}

func (sfu *SFU) consumerCatchup(catchupClientId string) {
	catchupClient := sfu.findClientById(catchupClientId)
	for _, client := range sfu.clients {
		clientId := client.Id()
		if clientId == catchupClientId {
			continue
		}
		for _, track := range client.Producer().GetMediaTracks() {
			catchupClient.AddConsumerTrack(catchupClientId, track)
		}
	}
}

func (sfu *SFU) handleProducerHandshake(data ArbiterTypes.HandshakePayload) {
	
	sender := data.Sender
	client := sfu.findClientById(sender)

	if client == nil {
		log.Println("Adding a new client")
		client = sfu.addClient(sender)
	}
	// how do we know if its a producer or consumer
	client.ProducerHandshake(data);
}

func (sfu *SFU) handleConsumerHandshake(data ArbiterTypes.HandshakePayload) {
	sender := data.Sender
	client := sfu.findClientById(sender)
	if client != nil {
		client.ConsumerHandshake(data)
	}
}

func (sfu *SFU) findClientById(clientId string) *Client.Client {
	client, ok := sfu.clients[clientId]
	if ok {
		return client
	}
	return nil
}

func (sfu *SFU) addClient(clientId string) *Client.Client {
	client := Client.NewClient(clientId, sfu.sfuId, sfu.rtcConfig, sfu.socket ,sfu.producerTrackChannel, sfu.featuresSharedChannel)
	sfu.clients[clientId] = client
	return client
}
