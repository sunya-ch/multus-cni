// Copyright (c) 2018 Intel Corporation
// Copyright (c) 2021 Multus Authors
//
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

package ext

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/logging"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/types"
	v1 "k8s.io/api/core/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/ext/proto"
)

const (
	generatedConfigType = "generated"
)

type GeneratedNetConf struct {
	types.NetConf
	Addr   string `json:"addr"`
	Config string `json:"config"`
}

func CheckGeneratedDelegate(config *types.DelegateNetConf) bool {
	return config.Conf.Type == generatedConfigType
}

func getGeneratedDelegatesFromResponse(response *pb.GenerateResponse, net *types.NetworkSelectionElement, resourceName string) ([]*types.DelegateNetConf, error) {
	var generatedDelegates []*types.DelegateNetConf
	if len(response.ConfList) == 0 {
		return nil, logging.Errorf("cannot get conflist: %v", response.Message)
	}
	if !response.Success {
		logging.Debugf("failed response: %v", response.Message)
	}
	for _, conf := range response.ConfList {
		// deviceID is supposed to be added to config by generator
		delegate, err := types.LoadDelegateNetConf(conf, net, "", resourceName)
		if err != nil {
			logging.Debugf("failed to LoadDelegateNetConf %s: %v", string(conf), err)
		}
		generatedDelegates = append(generatedDelegates, delegate)
	}
	return generatedDelegates, nil
}

func generateDelegates(conn grpc.ClientConnInterface, configRequest *pb.ConfigRequest, net *types.NetworkSelectionElement, resourceName string) ([]*types.DelegateNetConf, error) {
	c := pb.NewGeneratorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := c.Generate(ctx, configRequest)
	if err != nil {
		return nil, err
	}
	return getGeneratedDelegatesFromResponse(response, net, resourceName)
}

func cleanupDelegates(conn grpc.ClientConnInterface, configRequest *pb.ConfigRequest, net *types.NetworkSelectionElement, resourceName string) ([]*types.DelegateNetConf, error) {
	c := pb.NewGeneratorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := c.Cleanup(ctx, configRequest)
	if err != nil {
		return nil, err
	}
	return getGeneratedDelegatesFromResponse(response, net, resourceName)
}

func GenerateDelegates(config *types.DelegateNetConf, net *types.NetworkSelectionElement, pod *v1.Pod) ([]*types.DelegateNetConf, error) {
	var generatedConfig GeneratedNetConf
	err := json.Unmarshal(config.Bytes, &generatedConfig)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(generatedConfig.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, logging.Errorf("cannot connect to generator server: %v", err)
	}
	defer conn.Close()
	hostName, _ := os.Hostname()
	configRequest := &pb.ConfigRequest{
		PodName:      pod.Name,
		PodNamespace: pod.Namespace,
		HostName:     hostName,
		Data:         []byte(generatedConfig.Config),
	}
	if pod.ObjectMeta.GetDeletionTimestamp() != nil {
		// if the pod is going to be deleted, call Cleanup
		return cleanupDelegates(conn, configRequest, net, config.ResourceName)
	}
	// otherwise, call Generate
	return generateDelegates(conn, configRequest, net, config.ResourceName)
}
