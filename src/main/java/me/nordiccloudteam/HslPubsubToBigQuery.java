/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package me.nordiccloudteam;

/*
mvn compile exec:java     -Dexec.mainClass=me.nordiccloudteam.HslPubsubToBigQuery     -Dexec.args="--project=taneli-sandbox --outputDataset=hsl --outputTable=realtime --inputTopic=projects/taneli-sandbox/topics/hsl-realtime-data  --stagingLocation=gs://your-dataflow-bucket/staging --tempLocation=gs://your-dataflow-bucket/temp --runner=DirectRunner"
*/

import org.apache.beam.sdk.Pipeline;
import org.apache.beam.sdk.options.PipelineOptionsFactory;
import org.apache.beam.sdk.transforms.DoFn;
import org.apache.beam.sdk.transforms.ParDo;
import org.apache.beam.sdk.transforms.PTransform;
import org.apache.beam.sdk.values.KV;
import org.apache.beam.sdk.values.PCollection;

import com.google.api.services.bigquery.model.TableFieldSchema;
import com.google.api.services.bigquery.model.TableRow;
import com.google.api.services.bigquery.model.TableSchema;
import com.google.api.services.bigquery.model.TimePartitioning;

import java.io.StringReader;
import java.util.Map;
import java.util.ArrayList;
import java.util.List;
import org.joda.time.Duration;
import org.joda.time.Instant;
import com.google.gson.Gson;
import com.google.gson.JsonElement;

import org.apache.beam.sdk.io.gcp.bigquery.BigQueryIO;
import org.apache.beam.sdk.io.gcp.pubsub.PubsubIO;
import org.apache.beam.sdk.io.gcp.pubsub.PubsubMessage;
import org.apache.beam.sdk.io.redis.RedisConnectionConfiguration;
import org.apache.beam.sdk.transforms.windowing.FixedWindows;
import org.apache.beam.sdk.transforms.windowing.Window;

import io.suryawirawan.henry.beam.redis.io.CustomRedisIO;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 */

public class HslPubsubToBigQuery {
    private static final Logger LOG = LoggerFactory.getLogger(HslPubsubToBigQuery.class);

    static class PackJSONDataFn extends DoFn<TableRow, KV<String,String>> {
        @ProcessElement
        public void processElement(ProcessContext c) {
            TableRow row = c.element();
            Gson gson = new Gson();
            String redisKey = "line:" + (String)row.get("route_number") + ":" + row.get("direction").toString() + ":" + row.get("internal_journey").toString();
            String redisValue = gson.toJson(row);
            c.output(KV.<String,String>of(redisKey, redisValue));
        }
    }

    static class UnpackJSONDataFn extends DoFn<String, TableRow> {
        @ProcessElement
        public void processElement(ProcessContext c) {
            String data = c.element();
            try {
                Gson gson = new Gson();

                com.google.gson.JsonObject jsonObject = gson.fromJson(new StringReader(data), com.google.gson.JsonObject.class);
                com.google.gson.JsonObject jsonVp = jsonObject.getAsJsonObject("VP");
                
                TableRow row = new TableRow();

                String latitude = null, longitude = null;
                Instant rowTimestamp = Instant.now();
                for (Map.Entry<String, JsonElement> entry : jsonVp.entrySet()) {
                    com.google.gson.JsonElement element = entry.getValue();
                    if (element.isJsonNull()) // omit null keys in BQ
                        continue;

                    switch (entry.getKey()) {
                        case "desi":
                            row.put("route_number", element.getAsString());
                            break;
                        case "dir":
                            row.put("direction", element.getAsInt());
                            break;
                        case "oper":
                            row.put("operator", element.getAsInt());
                            break;
                        case "veh":
                            row.put("vehicle_number", element.getAsString());
                            break;
                        case "tst":
                            row.put("timestamp", element.getAsString());
                            rowTimestamp = Instant.parse(element.getAsString());
                            break;
                        case "tsi":
                            row.put("unix_time", element.getAsInt());
                            break;
                        case "spd":
                            row.put("speed", element.getAsFloat());
                            break;
                        case "hdg":
                            row.put("heading", element.getAsInt());
                            break;
                        case "lat":
                            latitude = element.getAsString();
                            break;
                        case "long":
                            longitude = element.getAsString();
                            break;
                        case "acc":
                            row.put("acceleration", element.getAsFloat());
                            break;
                        case "dl":
                            row.put("delayed", element.getAsInt());
                            break;
                        case "odo":
                            row.put("trip_odometer", element.getAsInt());
                            break;
                        case "drst":
                            row.put("doors_open", element.getAsInt());
                            break;
                        case "jrn":
                            row.put("internal_journey", element.getAsString());
                            break;
                        case "line":
                            row.put("internal_line", element.getAsString());
                            break;
                        case "start":
                            row.put("scheduled_start", element.getAsString() + ":00");
                            break;
                        case "oday":
                            row.put("operating_day", element.getAsString());
                            break;
                    }
                }
                if (latitude != null && longitude != null) {
                    row.put("location", "POINT(" + longitude + " " + latitude + ")");
                }
                c.outputWithTimestamp(row, c.timestamp());
            } catch (Exception e) {
                throw e;
            }
        }
    }

    static class HslDataTransform extends PTransform<PCollection<String>, PCollection<TableRow>> {

        @Override
        public PCollection<TableRow> expand(PCollection<String> data) {
            PCollection<TableRow> results = data.apply(ParDo.of(new UnpackJSONDataFn()));
            return results;
        }
    }

    public static void main(String[] args) {
        HslPubsubToBigQueryOptions options =
            PipelineOptionsFactory.fromArgs(args).withValidation().as(HslPubsubToBigQueryOptions.class);

        runImportHslData(options);
    }

    public static void runImportHslData(HslPubsubToBigQueryOptions options) {
        Pipeline p = Pipeline.create(options);
        String redisHost = null;
        int redisPort = 6379;

        if (options.getOutput().equals("redis")) { 
            redisHost = options.getRedisHost();
            redisPort = options.getRedisPort().intValue();
        }

        PCollection<TableRow> rows = p.apply(
            "ReadPubsubMessages",
            PubsubIO.readMessagesWithAttributes().fromTopic(options.getInputTopic())
        ).apply("PubsubMessagesToString",
            ParDo.of(new DoFn<PubsubMessage, String>() {
                @ProcessElement
                public void process(ProcessContext c) {
                    PubsubMessage msg = c.element();
                    c.output(new String(msg.getPayload()));
                }
            })
        ).apply(
            "ConvertMessageToTableRow", new HslDataTransform()
        );

        // Create schema
        List<TableFieldSchema> fields = new ArrayList<>();
        fields.add(new TableFieldSchema().setName("route_number").setType("STRING"));
        fields.add(new TableFieldSchema().setName("direction").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("operator").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("vehicle_number").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("timestamp").setType("TIMESTAMP"));
        fields.add(new TableFieldSchema().setName("unix_time").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("speed").setType("FLOAT64"));
        fields.add(new TableFieldSchema().setName("heading").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("location").setType("STRING"));
        fields.add(new TableFieldSchema().setName("acceleration").setType("FLOAT64"));
        fields.add(new TableFieldSchema().setName("delayed").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("trip_odometer").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("doors_open").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("operating_day").setType("DATE"));
        fields.add(new TableFieldSchema().setName("internal_journey").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("internal_line").setType("INTEGER"));
        fields.add(new TableFieldSchema().setName("scheduled_start").setType("TIME"));

        TableSchema schema = new TableSchema().setFields(fields);

        // Window data for BQ partitioning
        rows = rows/*.apply(Flatten.<TableRow>pCollections())*/
            .apply("FixedWindows",
            Window.<TableRow>into(FixedWindows.of(Duration.standardSeconds(5)))
        );

        if (options.getOutput().equals("bigquery")) {
            // Window data for BQ partitioning
            rows.apply(BigQueryIO
                .writeTableRows()
                .to(options.getProject() + ":" + options.getOutputDataset() + "." + options.getOutputTable())
                .withTimePartitioning(new TimePartitioning().setField("timestamp"))
                .withSchema(schema)
                .withWriteDisposition(BigQueryIO.Write.WriteDisposition.WRITE_APPEND)
                .withCreateDisposition(BigQueryIO.Write.CreateDisposition.CREATE_IF_NEEDED));
        } 
        if (options.getOutput().equals("redis")) {
            LOG.warn("Redis host is at: " + redisHost + ":" + redisPort);
            
            rows.apply(
                ParDo.of(new PackJSONDataFn())
            ).apply(CustomRedisIO.write().
                withEndpoint(redisHost, redisPort).
                withMethod(CustomRedisIO.Write.Method.LTRIM).
                withCappedCollectionSize(Long.valueOf(5)).
                withExpireTime(Long.valueOf(120 * 1000)));
        }
        p.run().waitUntilFinish();
    }
}

