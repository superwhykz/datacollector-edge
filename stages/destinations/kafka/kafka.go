// +build kafka

// Copyright 2018 StreamSets Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package kafka

import (
	"bytes"
	"context"
	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
	"github.com/streamsets/datacollector-edge/api"
	"github.com/streamsets/datacollector-edge/api/validation"
	"github.com/streamsets/datacollector-edge/container/common"
	"github.com/streamsets/datacollector-edge/container/el"
	"github.com/streamsets/datacollector-edge/stages/lib/datagenerator"
	"github.com/streamsets/datacollector-edge/stages/stagelibrary"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	APACHE_KAFKA_0_10_LIBRARY = "streamsets-datacollector-apache-kafka_0_10-lib"
	APACHE_KAFKA_0_11_LIBRARY = "streamsets-datacollector-apache-kafka_0_11-lib"
	APACHE_KAFKA_1_0_LIBRARY  = "streamsets-datacollector-apache-kafka_1_0-lib"
	APACHE_KAFKA_1_1_LIBRARY  = "streamsets-datacollector-apache-kafka_1_1-lib"

	CDH_KAFKA_2_0_LIBRARY = "streamsets-datacollector-cdh_kafka_2_0-lib"
	CDH_KAFKA_2_1_LIBRARY = "streamsets-datacollector-cdh_kafka_2_1-lib"
	CDH_KAFKA_3_0_LIBRARY = "streamsets-datacollector-cdh_kafka_3_0-lib"

	HDP_KAFKA_2_4_LIBRARY = "streamsets-datacollector-hdp_2_4-lib"
	HDP_KAFKA_2_5_LIBRARY = "streamsets-datacollector-hdp_2_5-lib"
	HDP_KAFKA_2_6_LIBRARY = "streamsets-datacollector-hdp_2_6-lib"

	StageName = "com_streamsets_pipeline_stage_destination_kafka_KafkaDTarget"

	SocketTimeoutMS                    = "socket.timeout.ms"
	SslEndpointIdentificationAlgorithm = "ssl.endpoint.identification.algorithm"
	SecurityProtocol                   = "security.protocol"
	SASLJaasConfig                     = "sasl.jaas.config"
	MessageMaxBytes                    = "message.max.bytes"
	RequestRequiredACKs                = "request.required.acks"
	RequestTimeoutMS                   = "request.timeout.ms"
	CompressionCodec                   = "compression.codec"
	QueueBufferingMaxMS                = "queue.buffering.max.ms"
	QueueBufferingMaxMessages          = "queue.buffering.max.messages"
	MessageSendMaxRetries              = "message.send.max.retries"
	RetryBackoffMS                     = "retry.backoff.ms"

	ClientId               = "SDCEdge"
	HTTPS                  = "https"
	SASLPlainText          = "SASL_PLAINTEXT"
	SASLSSL                = "SASL_SSL"
	SSSLJaasConfigRegex    = `.*username="(.*)".*password="(.*)"`
	CompressionCodecNone   = "none"
	CompressionCodecGzip   = "gzip"
	CompressionCodecSnappy = "snappy"
	CompressionCodecLz4    = "lz4"
)

type KafkaDestination struct {
	*common.BaseStage
	Conf            KafkaTargetConfig `ConfigDefBean:"conf"`
	kafkaClientConf *sarama.Config
	brokerList      []string
	kafkaClient     sarama.Client
	keyCounter      int
}

type KafkaTargetConfig struct {
	MetadataBrokerList        string                                  `ConfigDef:"type=STRING,required=true"`
	TopicWhiteList            string                                  `ConfigDef:"type=STRING,required=true"`
	Topic                     string                                  `ConfigDef:"type=STRING,required=true"`
	TopicExpression           string                                  `ConfigDef:"type=STRING,required=true,evaluation=EXPLICIT"`
	RuntimeTopicResolution    bool                                    `ConfigDef:"type=BOOLEAN,required=true"`
	PartitionStrategy         PartitionStrategy                       `ConfigDef:"type=MODEL,required=true"`
	SingleMessagePerBatch     bool                                    `ConfigDef:"type=BOOLEAN,required=true"`
	KafkaProducerConfigs      map[string]string                       `ConfigDef:"type=MAP,required=true"`
	DataFormat                string                                  `ConfigDef:"type=STRING,required=true"`
	DataGeneratorFormatConfig datagenerator.DataGeneratorFormatConfig `ConfigDefBean:"dataGeneratorFormatConfig"`
}

func init() {
	stagelibrary.SetCreator(APACHE_KAFKA_0_10_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(APACHE_KAFKA_0_11_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(APACHE_KAFKA_1_0_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(APACHE_KAFKA_1_1_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})

	stagelibrary.SetCreator(CDH_KAFKA_2_0_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(CDH_KAFKA_2_1_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(CDH_KAFKA_3_0_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})

	stagelibrary.SetCreator(HDP_KAFKA_2_4_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(HDP_KAFKA_2_5_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
	stagelibrary.SetCreator(HDP_KAFKA_2_6_LIBRARY, StageName, func() api.Stage {
		return &KafkaDestination{BaseStage: &common.BaseStage{}}
	})
}

func (dest *KafkaDestination) Init(context api.StageContext) []validation.Issue {
	issues := dest.BaseStage.Init(context)
	issues = dest.Conf.DataGeneratorFormatConfig.Init(dest.Conf.DataFormat, context, issues)

	var err error

	err = dest.mapJVMConfigsToSaramaConfig()

	dest.kafkaClientConf.Producer.Partitioner, err = getPartitionerConstructor(dest.Conf.PartitionStrategy)
	if err != nil {
		issues = append(issues, context.CreateConfigIssue(err.Error()))
		return issues
	}
	dest.brokerList = strings.Split(dest.Conf.MetadataBrokerList, ",")

	dest.kafkaClient, err = sarama.NewClient(dest.brokerList, dest.kafkaClientConf)
	if err != nil {
		issues = append(issues, context.CreateConfigIssue(err.Error()))
		return issues
	}

	dest.keyCounter = 0
	return issues
}

func (dest *KafkaDestination) Write(batch api.Batch) error {
	var err error

	kafkaProducer, err := sarama.NewAsyncProducerFromClient(dest.kafkaClient)
	if err != nil {
		return err
	}

	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.WithError(err).Error("Failed to close Kafka Producer")
		}
	}()

	recordWriterFactory := dest.Conf.DataGeneratorFormatConfig.RecordWriterFactory
	if err != nil {
		return err
	}

	go func() {
		for msg := range kafkaProducer.Successes() {
			log.WithFields(log.Fields{
				"key":       msg.Key,
				"topic":     msg.Topic,
				"partition": msg.Partition,
				"offset":    msg.Offset,
			}).Debug("Message delivered")
		}
	}()

	go func() {
		for err := range kafkaProducer.Errors() {
			log.WithFields(log.Fields{
				"key":       err.Msg.Key,
				"topic":     err.Msg.Topic,
				"partition": err.Msg.Partition,
				"offset":    err.Msg.Offset,
			}).WithError(err.Err).Error("Message delivery failed!")
		}
	}()

	// TODO: Support sending single message per batch -
	// SDCE-176 - Support sending single message per batch in Kafka destination

	for _, record := range batch.GetRecords() {
		recordContext := context.WithValue(context.Background(), el.RecordContextVar, record)

		recordBuffer := bytes.NewBuffer([]byte{})
		recordWriter, err := recordWriterFactory.CreateWriter(dest.GetStageContext(), recordBuffer)

		err = recordWriter.WriteRecord(record)
		if err != nil {
			return err
		}

		err = recordWriter.Flush()
		if err != nil {
			log.WithError(err).Error("Error flushing record writer")
		}

		err = recordWriter.Close()
		if err != nil {
			log.WithError(err).Error("Error closing record writer")
		}

		topic, err := resolveTopic(dest.GetStageContext(), recordContext, &dest.Conf)
		if err != nil {
			return err
		}

		log.Debug("Sending message")
		dest.keyCounter++
		kafkaProducer.Input() <- &sarama.ProducerMessage{
			Key:   sarama.StringEncoder(dest.Conf.Topic + strconv.Itoa(dest.keyCounter)),
			Topic: *topic,
			Value: sarama.ByteEncoder(recordBuffer.Bytes()),
		}
	}

	return nil
}

func (dest *KafkaDestination) Destroy() error {
	if dest.kafkaClient != nil && !dest.kafkaClient.Closed() {
		if err := dest.kafkaClient.Close(); err != nil {
			log.WithError(err).Error("Failed to close Kafka Client")
			return err
		}
	}
	return nil
}

func (dest *KafkaDestination) mapJVMConfigsToSaramaConfig() error {
	config := sarama.NewConfig()
	config.ClientID = ClientId

	for name, value := range dest.Conf.KafkaProducerConfigs {
		switch name {
		// NET Config
		case SocketTimeoutMS:
			if i, err := strconv.Atoi(value); err == nil {
				config.Net.DialTimeout = time.Duration(i) * time.Millisecond
				config.Net.ReadTimeout = time.Duration(i) * time.Millisecond
				config.Net.WriteTimeout = time.Duration(i) * time.Millisecond
			}
		case SslEndpointIdentificationAlgorithm:
			if value == HTTPS {
				config.Net.TLS.Enable = true
			}
		case SecurityProtocol:
			if value == SASLPlainText || value == SASLSSL {
				config.Net.SASL.Enable = true
			}
		case SASLJaasConfig:
			re := regexp.MustCompile(SSSLJaasConfigRegex)
			match := re.FindStringSubmatch(value)
			if len(match) > 2 {
				config.Net.SASL.User = match[1]
				config.Net.SASL.Password = match[2]
			}

		// Producer Config
		case MessageMaxBytes:
			if i, err := strconv.Atoi(value); err == nil {
				config.Producer.MaxMessageBytes = i
			}
		case RequestRequiredACKs:
			if i, err := strconv.Atoi(value); err == nil {
				switch i {
				case 0:
					config.Producer.RequiredAcks = sarama.NoResponse
				case 1:
					config.Producer.RequiredAcks = sarama.WaitForLocal
				case -1:
					config.Producer.RequiredAcks = sarama.WaitForAll
				}
			}
		case RequestTimeoutMS:
			if i, err := strconv.Atoi(value); err == nil {
				config.Producer.Timeout = time.Duration(i) * time.Millisecond
			}
		case CompressionCodec:
			switch value {
			case CompressionCodecNone:
				config.Producer.Compression = sarama.CompressionNone
			case CompressionCodecGzip:
				config.Producer.Compression = sarama.CompressionGZIP
			case CompressionCodecSnappy:
				config.Producer.Compression = sarama.CompressionSnappy
			case CompressionCodecLz4:
				config.Producer.Compression = sarama.CompressionLZ4
			}
		case QueueBufferingMaxMS:
			if i, err := strconv.Atoi(value); err == nil {
				config.Producer.Flush.Frequency = time.Duration(i) * time.Millisecond
			}
		case QueueBufferingMaxMessages:
			if i, err := strconv.Atoi(value); err == nil {
				config.Producer.Flush.MaxMessages = i
			}
		case MessageSendMaxRetries:
			if i, err := strconv.Atoi(value); err == nil {
				config.Producer.Retry.Max = i
			}
		case RetryBackoffMS:
			if i, err := strconv.Atoi(value); err == nil {
				config.Producer.Retry.Backoff = time.Duration(i) * time.Millisecond
			}
		}
	}

	dest.kafkaClientConf = config
	return dest.kafkaClientConf.Validate()
}

func resolveTopic(
	stageContext api.StageContext,
	recordContext context.Context,
	config *KafkaTargetConfig,
) (*string, error) {
	if !config.RuntimeTopicResolution {
		return &config.Topic, nil
	}

	result, err := stageContext.Evaluate(config.TopicExpression, "topicExpression", recordContext)
	if err != nil {
		return nil, err
	}

	topic := result.(string)
	return &topic, nil
}