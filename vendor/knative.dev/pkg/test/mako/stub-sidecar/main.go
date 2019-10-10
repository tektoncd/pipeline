/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/golang/protobuf/jsonpb"

	"google.golang.org/grpc"
	qspb "knative.dev/pkg/third_party/mako/proto/quickstore_go_proto"
)

const (
	port = ":9813"
	// A 10 minutes run at 1000 rps of eventing perf tests is usually ~= 70 MBi, so 100MBi is reasonable
	defaultServerMaxReceiveMessageSize = 1024 * 1024 * 100
)

type server struct {
	stopOnce sync.Once
	stopCh   chan struct{}
}

func (s *server) Store(ctx context.Context, in *qspb.StoreInput) (*qspb.StoreOutput, error) {
	m := jsonpb.Marshaler{}
	qi, _ := m.MarshalToString(in.GetQuickstoreInput())
	fmt.Printf("# %s\n", qi)
	cols := []string{"inputValue", "errorMessage"}
	writer := csv.NewWriter(os.Stdout)

	for _, sp := range in.GetSamplePoints() {
		for _, mv := range sp.GetMetricValueList() {
			cols = updateCols(cols, mv.GetValueKey())
			vals := map[string]string{"inputValue": fmt.Sprintf("%f", sp.GetInputValue())}
			vals[mv.GetValueKey()] = fmt.Sprintf("%f", mv.GetValue())
			writer.Write(makeRow(cols, vals))
		}
	}

	for _, ra := range in.GetRunAggregates() {
		cols = updateCols(cols, ra.GetValueKey())
		vals := map[string]string{ra.GetValueKey(): fmt.Sprintf("%f", ra.GetValue())}
		writer.Write(makeRow(cols, vals))
	}

	for _, sa := range in.GetSampleErrors() {
		vals := map[string]string{"inputValue": fmt.Sprintf("%f", sa.GetInputValue()), "errorMessage": sa.GetErrorMessage()}
		writer.Write(makeRow(cols, vals))
	}

	writer.Flush()
	fmt.Printf("# %s\n", strings.Join(cols, ","))

	return &qspb.StoreOutput{}, nil
}

func updateCols(prototype []string, k string) []string {
	for _, n := range prototype {
		if n == k {
			return prototype
		}
	}
	prototype = append(prototype, k)
	return prototype
}

func makeRow(prototype []string, points map[string]string) []string {
	row := make([]string, len(prototype))
	// n^2 but whatever
	for k, v := range points {
		for i, n := range prototype {
			if k == n {
				row[i] = v
			}
		}
	}
	return row
}

func (s *server) ShutdownMicroservice(ctx context.Context, in *qspb.ShutdownInput) (*qspb.ShutdownOutput, error) {
	s.stopOnce.Do(func() { close(s.stopCh) })
	return &qspb.ShutdownOutput{}, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.MaxRecvMsgSize(defaultServerMaxReceiveMessageSize))
	stopCh := make(chan struct{})
	go func() {
		qspb.RegisterQuickstoreServer(s, &server{stopCh: stopCh})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	<-stopCh
	s.GracefulStop()
}
