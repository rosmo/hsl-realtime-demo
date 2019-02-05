## Places you need to edit

- configmap.yaml specifies configuration parameters for the frontend app
- message-broker.yaml for correct image locations
- app/frontend.yaml for correct image locations and Redis host location

## Deploying

You will need to deploy 4 components:
- Message broker, a Go application which reads from mqtt.hsl.fi:443 and writes the messages to Pub/Sub
- Two Dataflow pipelines, one which writes information from Pub/Sub to Memorystore (Redis) 
  and one which writes it to BigQuery (same pipeline, different modes)
- Frontend application with the necessary HTML/JS/CSS to make it pretty

You'll also want to have a nice frontend URL pointing to the app.

## Build containers

```
docker build -t eu.gcr.io/YOUR-PROJECT/hsl-realtime-demo-message-broker:v1 .
cd app && docker build -t eu.gcr.io/YOUR-PROJECT/hsl-realtime-demo-frontend:v1 .
```

(push images to repo)

## Deploying to Google Kubernetes Engine

You will need to enable BigQuery and Pub/Sub when creating the cluster and the native networking
to talk to Redis. Alternatively for enabling BigQuery & Pub/Sub globally for a cluster, you
can use Kelsey Hightower's tutorial to mount individual service accounts for the pods:
https://github.com/kelseyhightower/gke-service-accounts-tutorial


Use `kubectl create -f` for configmap.yaml, message-broker.yaml, app/frontend.yaml.

## Running the pipelines

### Running a pipeline locally

```
mvn compile exec:java  -e   -Dexec.mainClass=me.nordiccloudteam.HslPubsubToBigQuery     -Dexec.args="--project=YOUR-PROJECT --outputDataset=hsl --outputTable=realtime --inputTopic=projects/YOUR-PROJECT/topics/hsl-realtime-data  --stagingLocation=gs://YOUR-BUCKET/staging --tempLocation=gs://YOUR-BUCKET/temp --output=redis --redisHost=localhost --redisPort=6379 --runner=DirectRunner"
```

### Running the pipelines in Dataflow (Memorystore in zone europe-north1-a)

```
mvn compile exec:java  -e   -Dexec.mainClass=me.nordiccloudteam.HslPubsubToBigQuery     -Dexec.args--project=YOUR-PROJECT --outputDataset=hsl --outputTable=realtime --inputTopic=projects/YOUR-PROJECt/topics/hsl-realtime-data  --stagingLocation=gs://YOUR-BUCKET/staging --tempLocation=gs://YOUR-BUCKET/temp --output=redis --redisHost=MEMORYSTORE-IP --redisPort=6379 --region=europe-west1 --zone=europe-north1-a --jobName=pubsub-to-redis --runner=DataflowRunner"

mvn compile exec:java  -e   -Dexec.mainClass=me.nordiccloudteam.HslPubsubToBigQuery     -Dexec.args="--project=YOUR-PROJECT --outputDataset=hsl --outputTable=realtime --inputTopic=projects/YOUR-PROJECT/topics/hsl-realtime-data  --stagingLocation=gs://YOUR-BUCKET/staging --tempLocation=gs://YOUR-BUCKET/temp --output=bigquery  --region=europe-west1 --zone=europe-north1-a --jobName=pubsub-to-bigquery --runner=DataflowRunner"
```

Feel free to change regions/zones.

## BigQuery query to get last location

```
SELECT
  route_number,
  timestamp,
  speed,
  ST_GeogFromText(location) AS geo
FROM (
  SELECT
    route_number,
    timestamp,
    location,
    IFNULL(speed, 0) AS speed,
    ROW_NUMBER() OVER (PARTITION BY route_number ORDER BY timestamp DESC) AS row_num
  FROM
    hsl.realtime
  WHERE
    location IS NOT NULL
    AND DATETIME(timestamp) > DATETIME_SUB(CURRENT_DATETIME(),
      INTERVAL 10 MINUTE) )
WHERE
  row_num = 1;
```

Assume you have a table containing `(name STRING, area GEOGRAPHY)` in table `traffic_areas`, get the
speed of traffic in those areas (as defined with polygons) for the last hour:

```
SELECT ROW_NUMBER() OVER() AS row_number, name, ANY_VALUE(area) AS area, AVG(speed) AS average_speed FROM 
(SELECT 
  a.name, a.area, speed
FROM hsl.traffic_areas AS a
CROSS JOIN
 hsl.realtime AS b
WHERE
 b.location IS NOT NULL AND b.timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP, INTERVAL 1 HOUR) AND ST_WITHIN(ST_GeogFromText(b.location), a.area)
)
GROUP BY 2;
```

```
SELECT ROW_NUMBER() OVER() AS row_number, name, ANY_VALUE(area) AS area, AVG(speed) AS average_speed FROM 
(SELECT 
  a.name, a.area, speed
FROM hsl.traffic_areas AS a
CROSS JOIN
 hsl.realtime AS b
WHERE
 b.location IS NOT NULL AND b.timestamp >= TIMESTAMP_SUB((SELECT MAX(timestamp) FROM hsl.realtime), INTERVAL 1 HOUR) AND ST_WITHIN(ST_GeogFromText(b.location), a.area)
)
GROUP BY 2;
```

You can use Google Maps to draw the polygons, export to KML and use the `kms-to-bigquery.py`
script to translate the polygons into WKT format.

## Components used

- Animated Marker Movement: Robert Gerlach 2012-2013 https://github.com/combatwombat/marker-animate (MIT license)
- jQuery: (c) JS Foundation and other contributors (MIT license)
- jQuery Easing: Copyright © 2008 George McGinley Smith (BSD license)
- SlidingMarker: 19-02-2016 (C) 2015 Terikon Apps (MIT license)
- Google I/O 2017 transport tracker: https://github.com/googlemaps/transport-tracker (Apache License 2.0)
- Example MQTT code borrowed from: https://github.com/yosssi/gmq (MIT license)
- Custom RedisIO code from: https://github.com/henryken/beam-custom-redis-io (Apache License 2.0)