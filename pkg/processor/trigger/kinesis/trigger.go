/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kinesis

import (
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	kinesisclient "github.com/sendgridlabs/go-kinesis"
)

type kinesis struct {
	trigger.AbstractTrigger
	configuration *Configuration
	kinesisAuth   kinesisclient.Auth
	kinesisClient kinesisclient.KinesisClient
	shards        []*shard
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {
	instanceLogger := parentLogger.GetChild(configuration.ID)

	abstractTrigger, err := trigger.NewAbstractTrigger(instanceLogger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"kinesis",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := &kinesis{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
	}
	newTrigger.AbstractTrigger.Trigger = newTrigger
	newTrigger.kinesisAuth = kinesisclient.NewAuth(configuration.AccessKeyID,
		configuration.SecretAccessKey,
		"")

	switch endpoint := configuration.URL; {
	case endpoint != "":
		newTrigger.kinesisClient = kinesisclient.NewWithEndpoint(newTrigger.kinesisAuth,
			configuration.RegionName,
			endpoint)
	default:
		newTrigger.kinesisClient = kinesisclient.New(newTrigger.kinesisAuth, configuration.RegionName)
	}

	// iterate over shards and create
	for _, shardID := range configuration.Shards {

		// create the shard
		shard, err := newShard(newTrigger.Logger, newTrigger, shardID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create shard")
		}

		// add shard
		newTrigger.shards = append(newTrigger.shards, shard)
	}

	return newTrigger, nil
}

func (k *kinesis) Start(checkpoint functionconfig.Checkpoint) error {
	k.Logger.InfoWith("Starting",
		"streamName", k.configuration.StreamName,
		"shards", k.configuration.Shards)

	for _, shardInstance := range k.shards {

		// start reading from shard
		go func(shardInstance *shard) {
			if err := shardInstance.readFromShard(); err != nil {
				k.Logger.ErrorWith("Failed to read from shard", "err", err)
			}
		}(shardInstance)
	}

	return nil
}

func (k *kinesis) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (k *kinesis) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}
