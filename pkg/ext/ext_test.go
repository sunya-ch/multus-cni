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
	"encoding/json"
	"fmt"
	pb "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/ext/proto"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/types"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

func getGeneratedConfig(requestConfig string) (*types.DelegateNetConf, GeneratedNetConf) {
	conf := fmt.Sprintf(`{
		"name": "generated-network",
		"type": "generated",
		"addr": "fake-addr:fake-port",
		"config": "%s"
	}
	`, requestConfig)

	delegate, err := types.LoadDelegateNetConf([]byte(conf), nil, "0000:00:00.0", "")
	var generatedConfig GeneratedNetConf
	err = json.Unmarshal(delegate.Bytes, &generatedConfig)
	Expect(err).NotTo(HaveOccurred())
	return delegate, generatedConfig
}

func testGenerator(requestConfig string) ([]*types.DelegateNetConf, []*types.DelegateNetConf) {
	delegate, generatedConfig := getGeneratedConfig(requestConfig)
	Expect(CheckGeneratedDelegate(delegate)).To(BeTrue())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(err).NotTo(HaveOccurred())
	configRequest := &pb.ConfigRequest{
		Data: []byte(generatedConfig.Config),
	}
	generatedDelegates, err := generateDelegates(conn, configRequest, nil, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(len(generatedDelegates)).To(Equal(configBytesSize))
	cleanupDelegates, err := cleanupDelegates(conn, configRequest, nil, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(len(cleanupDelegates)).To(Equal(configBytesSize))
	return generatedDelegates, cleanupDelegates
}

func testSimpleGeneratedDelegate(generatedDelegates []*types.DelegateNetConf) {
	for _, generatedConf := range generatedDelegates {
		Expect(generatedConf.Conf.Type).To(Equal("fake-type"))
	}
}

func testGeneratedChainedDelegate(generatedDelegates []*types.DelegateNetConf) {
	for _, generatedConf := range generatedDelegates {
		Expect(generatedConf.Conf.Type).To(Equal(""))
		plugins := generatedConf.ConfList.Plugins
		Expect(len(plugins)).To(Equal(2))
		Expect(plugins[0].Type).To(Equal("fake-type"))
		Expect(plugins[1].Type).To(Equal("chained-type"))
	}
}

var fakePod = &v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "fakePod",
		Namespace: "podNamespace",
	},
}

var _ = Describe("Test GenerateDelegates", func() {
	It("generate with simple config", func() {
		requestConfig := `{\"type\": \"fake-type\"}`
		generatedDelegates, cleanupDelegates := testGenerator(requestConfig)
		testSimpleGeneratedDelegate(generatedDelegates)
		testSimpleGeneratedDelegate(cleanupDelegates)
	})
	It("generate with chained plugins", func() {
		requestConfig := `{\"plugins\": [{\"type\": \"fake-type\"}, {\"type\": \"chained-type\"}]}`
		generatedDelegates, cleanupDelegates := testGenerator(requestConfig)
		testGeneratedChainedDelegate(generatedDelegates)
		testGeneratedChainedDelegate(cleanupDelegates)
	})
})
