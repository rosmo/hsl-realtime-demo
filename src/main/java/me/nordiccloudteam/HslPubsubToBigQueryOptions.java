package me.nordiccloudteam;

import org.apache.beam.sdk.annotations.*;
import org.apache.beam.sdk.options.PipelineOptions;
import org.apache.beam.sdk.extensions.gcp.options.GcpOptions;

public interface HslPubsubToBigQueryOptions extends PipelineOptions, GcpOptions {

    //@Description("Pubsub topic to read from")
    //@Default.String("project/...")
    String getInputTopic();
    void setInputTopic(String value);

    //@Description("Write to bigquery or redis")
    //@Default.String("bigquery")
    String getOutput();
    void setOutput(String value);

    //@Description("BQ table to write to")
    String getOutputTable();
    void setOutputTable(String value);

    //@Description("BQ dataset to write to")
    String getOutputDataset();
    void setOutputDataset(String value);

    //@Description("Redis host")
    String getRedisHost();
    void setRedisHost(String value);

    //@Description("Redis port")
    Integer getRedisPort();
    void setRedisPort(Integer value);
    
}