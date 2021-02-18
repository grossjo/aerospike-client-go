// Copyright 2014-2021 Aerospike, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aerospike

import (
	"fmt"

	. "github.com/grossjo/aerospike-client-go/logger"
	. "github.com/grossjo/aerospike-client-go/types"
	Buffer "github.com/grossjo/aerospike-client-go/utils/buffer"
)

// guarantee touchCommand implements command interface
var _ command = &touchCommand{}

type touchCommand struct {
	singleCommand

	policy *WritePolicy
}

func newTouchCommand(cluster *Cluster, policy *WritePolicy, key *Key) (touchCommand, error) {
	partition, err := PartitionForWrite(cluster, &policy.BasePolicy, key)
	if err != nil {
		return touchCommand{}, err
	}

	newTouchCmd := touchCommand{
		singleCommand: newSingleCommand(cluster, key, partition),
		policy:        policy,
	}

	return newTouchCmd, nil
}

func (cmd *touchCommand) getPolicy(ifc command) Policy {
	return cmd.policy
}

func (cmd *touchCommand) writeBuffer(ifc command) error {
	return cmd.setTouch(cmd.policy, cmd.key)
}

func (cmd *touchCommand) getNode(ifc command) (*Node, error) {
	return cmd.partition.GetNodeWrite(cmd.cluster)
}

func (cmd *touchCommand) prepareRetry(ifc command, isTimeout bool) bool {
	cmd.partition.PrepareRetryWrite(isTimeout)
	return true
}

func (cmd *touchCommand) parseResult(ifc command, conn *Connection) error {
	// Read header.
	_, err := conn.Read(cmd.dataBuffer, 8)
	if err != nil {
		return err
	}

	if compressedSize := cmd.compressedSize(); compressedSize > 0 {
		// Read compressed size
		_, err = conn.Read(cmd.dataBuffer, compressedSize)
		if err != nil {
			Logger.Debug("Connection error reading data for TouchCommand: %s", err.Error())
			return err
		}

		// Read compressed size
		_, err = conn.Read(cmd.dataBuffer, 8)
		if err != nil {
			Logger.Debug("Connection error reading data for TouchCommand: %s", err.Error())
			return err
		}

		if err := cmd.conn.initInflater(true, compressedSize); err != nil {
			return NewAerospikeError(PARSE_ERROR, fmt.Sprintf("Error setting up zlib inflater for size `%d`: %s", compressedSize, err.Error()))
		}

		// Read header.
		_, err = conn.Read(cmd.dataBuffer, int(_MSG_TOTAL_HEADER_SIZE))
		if err != nil {
			Logger.Debug("Connection error reading data for TouchCommand: %s", err.Error())
			return err
		}
	} else {
		// Read header.
		_, err = conn.Read(cmd.dataBuffer[8:], int(_MSG_TOTAL_HEADER_SIZE)-8)
		if err != nil {
			Logger.Debug("Connection error reading data for TouchCommand: %s", err.Error())
			return err
		}
	}

	// Read header.
	header := Buffer.BytesToInt64(cmd.dataBuffer, 0)

	// Validate header to make sure we are at the beginning of a message
	if err := cmd.validateHeader(header); err != nil {
		return err
	}

	resultCode := cmd.dataBuffer[13] & 0xFF

	if resultCode != 0 {
		if resultCode == byte(KEY_NOT_FOUND_ERROR) {
			return ErrKeyNotFound
		} else if ResultCode(resultCode) == FILTERED_OUT {
			return ErrFilteredOut
		}

		return NewAerospikeError(ResultCode(resultCode))
	}
	if err := cmd.emptySocket(conn); err != nil {
		return err
	}
	return nil
}

func (cmd *touchCommand) Execute() error {
	return cmd.execute(cmd, false)
}
