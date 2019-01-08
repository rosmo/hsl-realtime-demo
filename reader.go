package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/certifi/gocertifi"

	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"

	"cloud.google.com/go/pubsub"

	"flag"
	"strconv"
)

func main() {
	var projectID string
	var topicName string
	var geohashLevel int = 0

	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	ctx := context.Background()

	var _projectID = flag.String("project", "", "set GCP project id")
	var _topicName = flag.String("topic", "", "set Pub/Sub topic (default hsl-realtime-data)")
	var stripVP = flag.Bool("strip", false, "strip VP1 from JSON data (default false)")
	var debug = flag.Bool("debug", false, "print messages and debug info")
	flag.IntVar(&geohashLevel, "geohash", 0, "specify geohash level (default=0)")
	flag.Parse()

	if *_projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	} else {
		projectID = *_projectID
	}
	if projectID == "" {
		panic("Project ID needs to be set in environment variable GOOGLE_CLOUD_PROJECT or through -project flag")
	}

	if *_topicName == "" {
		topicName = os.Getenv("PUBSUB_TOPIC")
	} else {
		topicName = *_topicName
	}
	if topicName == "" {
		topicName = "hsl-realtime-data"
	}

	if os.Getenv("GEOHASH_LEVEL") != "" {
		geohashLevel, _ = strconv.Atoi(os.Getenv("GEOHASH_LEVEL"))
	}

	psCli, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	if *debug {
		fmt.Printf("Using topic: %s\n", topicName)
	}

	// Creates the new topic.
	psTopic, err := psCli.CreateTopic(ctx, topicName)
	if err != nil {
		psTopic = psCli.Topic(topicName)
	}

	// Create an MQTT Client.
	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			fmt.Println(err)
			panic(err)
		},
	})

	// Terminate the Client.
	defer cli.Terminate()

	cert_pool, err := gocertifi.CACerts()

	tlsConfig := &tls.Config{
		RootCAs: cert_pool,
	}

	// Connect to the MQTT Server.
	err = cli.Connect(&client.ConnectOptions{
		// Network is the network on which the Client connects to.
		Network: "tcp",
		// Address is the address which the Client connects to.
		Address: "mqtt.hsl.fi:443",
		// TLSConfig is the configuration for the TLS connection.
		// If this property is not nil, the Client tries to use TLS
		// for the connection.
		TLSConfig:    tlsConfig,
		CleanSession: true,
	})

	if err != nil {
		panic(err)
	}

	var topicSubscription = "/hfp/v1/journey/ongoing/+/+/+/+/+/+/+/+/" + strconv.Itoa(geohashLevel) + "/#"
	// Subscribe to topics.
	err = cli.Subscribe(&client.SubscribeOptions{
		SubReqs: []*client.SubReq{
			&client.SubReq{
				TopicFilter: []byte(topicSubscription),
				QoS:         mqtt.QoS0,
				// Define the processing of the message handler.
				Handler: func(topicName, message []byte) {
					if *stripVP {
						message = message[6 : len(message)-1]
					}
					if *debug {
						fmt.Println(string(message))
					} else {
						fmt.Print(".")
					}

					psMessage := new(pubsub.Message)
					psMessage.Data = message
					psMessage.Attributes = map[string]string{"originalTopic": string(topicName)}

					if publishResult := psTopic.Publish(ctx, psMessage); publishResult == nil {
						log.Fatalf("Failed to publish: %v", err)
					}
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Wait for receiving a signal.
	<-sigc

	// Disconnect the Network Connection.
	if err := cli.Disconnect(); err != nil {
		panic(err)
	}
}
