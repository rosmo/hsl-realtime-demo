package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/certifi/gocertifi"

	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"

	"cloud.google.com/go/pubsub"

	"flag"
	"strconv"
	"strings"
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
	var _simulate = flag.String("simulate", "", "set filename to read simulation data from")
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

	if *_simulate != "" {
		file, err := os.Open(*_simulate)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		line := 0
		scanner := bufio.NewScanner(file)
		var timer int32 = int32(time.Now().Unix())
		var first_time int32 = 0
		for scanner.Scan() {
			if line > 0 {
				var tst string = "TIMESTAMP"
				var tsi string = "unixtime"

				s := scanner.Text()
				ss := strings.Split(s, ",")
				if first_time == 0 {
					_first_time, _ := strconv.Atoi(ss[6])
					first_time = int32(_first_time)
				}
				_check, _ := strconv.Atoi(ss[6])
				if int32(_check) != first_time {
					// wait until next second
					for int32(time.Now().Unix())-timer == 0 {
						time.Sleep(time.Duration(50) * time.Microsecond)
					}
					timer = int32(time.Now().Unix())
					_first_time, _ := strconv.Atoi(ss[6])
					first_time = int32(_first_time)
				}

				latlon := strings.Split(ss[8][6:len(ss[8])-1], " ")
				var lat string = latlon[1]
				var lon string = latlon[0]
				tsi = fmt.Sprintf("%d", timer)
				unixTimeUTC := time.Unix(int64(timer), 0)
				tst = unixTimeUTC.Format("2006-01-02T15:04:05Z")

				odo, _ := strconv.Atoi(ss[11])
				drst, _ := strconv.Atoi(ss[12])
				dl, _ := strconv.Atoi(ss[10])
				spd, _ := strconv.ParseFloat(ss[6], 64)
				acc, _ := strconv.ParseFloat(ss[9], 64)
				msg := fmt.Sprintf("{\"VP\":{\"desi\":\"%s\",\"dir\":\"%s\",\"oper\":\"%s\",\"veh\":\"%s\",\"tst\":\"%s\",\"tsi\":\"%s\",\"spd\":\"%f\",\"hdg\":\"%s\",\"lat\":\"%s\",\"long\":\"%s\",\"acc\":\"%f\",\"dl\":\"%d\",\"odo\":\"%d\",\"drst\":\"%d\",\"oday\":\"%s\",\"jrn\":\"%s\",\"line\":\"%s\",\"start\":\"%s\"}}", ss[0], ss[1], ss[2], ss[3], tst, tsi, spd, ss[7], lat, lon, acc, dl, odo, drst, ss[13], ss[14], ss[15], ss[16])
				for loop := 0; loop < 4; loop++ {
					psMessage := new(pubsub.Message)
					psMessage.Data = []byte(msg)
					psMessage.Attributes = map[string]string{"originalTopic": string(topicName)}
					if publishResult := psTopic.Publish(ctx, psMessage); publishResult == nil {
						log.Fatalf("Failed to publish: %v", err)
					}
				}

				fmt.Println(msg)
			}
			line = line + 1
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		panic("I'm done")
	}

	cert_pool, err := gocertifi.CACerts()

	tlsConfig := &tls.Config{
		RootCAs: cert_pool,
	}

	// Connect to the MQTT Server.
	err = cli.Connect(&client.ConnectOptions{
		// Network is the network on which the Client connects to.
		Network: "tcp",
		// Address is the address which the Client connects to.
		Address: "mqtt.hsl.fi:8883",
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
