// Copyright (c) 2022 Multus Authors
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
	"log"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pb "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/ext/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestGeneratedConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "config generator")
}

// dummy server is used to implement GeneratorServer.
type server struct {
	pb.UnimplementedGeneratorServer
}

const (
	configBytesSize = 2
	expectedMessage = "succeeded"
)

var stopGeneratorServer func()
var listener *bufconn.Listener

// getIdenticalConfigBytes always returns two identical config bytes
func getIdenticalConfigBytes(configRequest *pb.ConfigRequest) [][]byte {
	configBytesSize := 2
	confBytesArray := [][]byte{}
	for index := 0; index < configBytesSize; index++ {
		confBytesArray = append(confBytesArray, configRequest.Data)
	}
	return confBytesArray
}

// dummy Generate
func (s *server) Generate(ctx context.Context, configRequest *pb.ConfigRequest) (*pb.GenerateResponse, error) {
	confBytesArray := getIdenticalConfigBytes(configRequest)
	return &pb.GenerateResponse{Success: true, ConfList: confBytesArray, Message: expectedMessage}, nil
}

// dummy Cleanup
func (s *server) Cleanup(ctx context.Context, configRequest *pb.ConfigRequest) (*pb.GenerateResponse, error) {
	confBytesArray := getIdenticalConfigBytes(configRequest)
	return &pb.GenerateResponse{Success: true, ConfList: confBytesArray, Message: expectedMessage}, nil
}

var _ = BeforeSuite(func() {
	buffer := 101024 * 1024
	listener = bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	pb.RegisterGeneratorServer(baseServer, &server{})

	stopGeneratorServer = func() {
		err := listener.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}

	go func() {
		if err := baseServer.Serve(listener); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()
})

var _ = AfterSuite(func() {
	By("Stop generator gRPC server")
	stopGeneratorServer()
})
