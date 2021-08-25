// Licensed to LinDB under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. LinDB licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package replica

import (
	"bytes"
	"errors"
	"io/ioutil"
	"sync"

	"github.com/golang/snappy"

	"github.com/lindb/lindb/constants"
	"github.com/lindb/lindb/pkg/logger"
	protoMetricsV1 "github.com/lindb/lindb/proto/gen/v1/metrics"
	"github.com/lindb/lindb/tsdb"
)

var snappyReaderPool = sync.Pool{
	New: func() interface{} {
		return snappy.NewReader(nil)
	},
}

type localReplicator struct {
	replicator

	shard  tsdb.Shard
	logger *logger.Logger
}

func NewLocalReplicator(channel *ReplicatorChannel, shard tsdb.Shard) Replicator {
	return &localReplicator{
		replicator: replicator{
			channel: channel,
		},
		shard:  shard,
		logger: logger.GetLogger("replica", "localReplicator"),
	}
}

func (r *localReplicator) Replica(_ int64, msg []byte) {
	//TODO add util
	reader := snappyReaderPool.Get().(*snappy.Reader)
	reader.Reset(bytes.NewReader(msg))
	defer func() {
		reader.Reset(nil)
		snappyReaderPool.Put(reader)
	}()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		r.logger.Error("decompress replica data error", logger.Error(err))
		return
	}
	var metricList protoMetricsV1.MetricList
	err = metricList.Unmarshal(data)
	if err != nil {
		r.logger.Error("unmarshal metricList", logger.Error(err))
		return
	}

	//TODO writeTask metric, need handle panic
	for _, metric := range metricList.Metrics {
		if err := r.shard.Write(metric); err != nil {
			//TODO need move broker
			if errors.Is(err, constants.ErrMetricOutOfTimeRange) {
				continue
			}
			r.logger.Error("writeTask metric", logger.Error(err))
		}
	}
}
