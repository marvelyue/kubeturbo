/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

// Package app implements a Server object for running the scheduler.
package app

import (
	"net"
	// "net/http"
	"os"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	// "k8s.io/kubernetes/pkg/healthz"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/client/record"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	kubeturbo "github.com/vmturbo/kubeturbo/pkg"
	"github.com/vmturbo/kubeturbo/pkg/conversion"
	"github.com/vmturbo/kubeturbo/pkg/metadata"
	"github.com/vmturbo/kubeturbo/pkg/registry"
	"github.com/vmturbo/kubeturbo/pkg/storage"
	etcdhelper "github.com/vmturbo/kubeturbo/pkg/storage/etcd"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

const (
	// The default port for vmt service server
	VMTPort = 10265

	DefaultEtcdPathPrefix = "/registry"
)

// VMTServer has all the context and params needed to run a Scheduler
type VMTServer struct {
	Port                  int
	Address               net.IP
	Master                string
	MetaConfigPath        string
	Kubeconfig            string
	BindPodsQPS           float32
	BindPodsBurst         int
	EtcdServerList        []string
	EtcdCA                string
	EtcdClientCertificate string
	EtcdClientKey         string
	EtcdConfigFile        string
	EtcdPathPrefix        string

	LeaderElection componentconfig.LeaderElectionConfiguration
}

// NewVMTServer creates a new VMTServer with default parameters
func NewVMTServer() *VMTServer {
	s := VMTServer{
		Port:    VMTPort,
		Address: net.ParseIP("127.0.0.1"),
	}
	return &s
}

// AddFlags adds flags for a specific VMTServer to the specified FlagSet
func (s *VMTServer) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&s.Port, "port", s.Port, "The port that the scheduler's http service runs on")
	fs.StringVar(&s.Master, "master", s.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.MetaConfigPath, "config-path", s.MetaConfigPath, "The path to the vmt config file.")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.StringSliceVar(&s.EtcdServerList, "etcd-servers", s.EtcdServerList, "List of etcd servers to watch (http://ip:port), comma separated. Mutually exclusive with -etcd-config")
	fs.StringVar(&s.EtcdCA, "cacert", s.EtcdCA, "Path to etcd ca.")
	fs.StringVar(&s.EtcdClientCertificate, "client-cert", s.EtcdClientCertificate, "Path to etcd client certificate")
	fs.StringVar(&s.EtcdClientKey, "client-key", s.EtcdClientKey, "Path to etcd client key")
	leaderelection.BindFlags(&s.LeaderElection, fs)
}

// Run runs the specified VMTServer.  This should never exit.
func (s *VMTServer) Run(_ []string) error {
	if s.Kubeconfig == "" && s.Master == "" {
		glog.Warningf("Neither --kubeconfig nor --master was specified.  Using default API client.  This might not work.")
	}

	glog.V(3).Infof("Master is %s", s.Master)

	if s.MetaConfigPath == "" {
		glog.Fatalf("The path to the VMT config file is not provided.Exiting...")
		os.Exit(1)
	}

	if (s.EtcdConfigFile != "" && len(s.EtcdServerList) != 0) || (s.EtcdConfigFile == "" && len(s.EtcdServerList) == 0) {
		glog.Fatalf("specify either --etcd-servers or --etcd-config")
	}

	// This creates a client, first loading any specified kubeconfig
	// file, and then overriding the Master flag, if non-empty.
	// kubeconfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
	// 	&clientcmd.ClientConfigLoadingRules{ExplicitPath: s.Kubeconfig},
	// 	&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: s.Master}}).ClientConfig()

	kubeconfig, err := clientcmd.BuildConfigFromFlags(s.Master, s.Kubeconfig)
	if err != nil {
		glog.Errorf("Error getting kubeconfig:  %s", err)
		return err
	}
	// This specifies the number and the max number of query per second to the api server.
	kubeconfig.QPS = 20.0
	kubeconfig.Burst = 30

	kubeClient, err := client.New(kubeconfig)
	if err != nil {
		glog.Fatalf("Invalid API configuration: %v", err)
	}

	// TODO not clear
	// go func() {
	// 	mux := http.NewServeMux()
	// 	healthz.InstallHandler(mux)
	// 	if s.EnableProfiling {
	// 		mux.HandleFunc("/debug/pprof/", pprof.Index)
	// 		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	// 		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	// 	}
	// 	mux.Handle("/metrics", prometheus.Handler())

	// 	server := &http.Server{
	// 		Addr:    net.JoinHostPort(s.Address.String(), strconv.Itoa(s.Port)),
	// 		Handler: mux,
	// 	}
	// 	glog.Fatal(server.ListenAndServe())
	// }()

	// serverAddr, targetType, nameOrAddress, targetIdentifier, password
	vmtMeta, err := metadata.NewVMTMeta(s.MetaConfigPath)
	if err != nil {
		glog.Errorf("Get error when loading configurations: %s", err)
		os.Exit(1)
	}
	glog.V(3).Infof("Finished loading configuration from %s", s.MetaConfigPath)

	etcdclientBuilder := etcdhelper.NewEtcdClientBuilder().ServerList(s.EtcdServerList).SetTransport(s.EtcdCA, s.EtcdClientCertificate, s.EtcdClientKey)
	etcdClient, err := etcdclientBuilder.CreateAndTest()
	if err != nil {
		glog.Errorf("Error creating etcd client instance for vmt service: %s", err)
		return err
	}

	s.EtcdPathPrefix = DefaultEtcdPathPrefix
	etcdStorage, err := newEtcd(etcdClient, s.EtcdPathPrefix)
	if err != nil {
		glog.Warningf("Error creating etcd storage instance for vmt service: %s", err)
		return err
	}

	vmtConfig := kubeturbo.NewVMTConfig(kubeClient, etcdStorage, vmtMeta)

	eventBroadcaster := record.NewBroadcaster()
	vmtConfig.Recorder = eventBroadcaster.NewRecorder(api.EventSource{Component: "kubeturbo"})
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(kubeClient.Events(""))

	vmtService := kubeturbo.NewVMTurboService(vmtConfig)

	run := func(_ <-chan struct{}) {
		vmtService.Run()
		select {}
	}

	if !s.LeaderElection.LeaderElect {
		glog.Infof("No leader election")
		run(nil)
		glog.Fatal("this statement is unreachable")
		panic("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		EndpointsMeta: api.ObjectMeta{
			Namespace: "kube-system",
			Name:      "kubeturbo",
		},
		Client:        kubeClient,
		Identity:      id,
		EventRecorder: vmtConfig.Recorder,
		LeaseDuration: s.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: s.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   s.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				glog.Fatalf("lost master")
			},
		},
	})

	glog.Fatal("this statement is unreachable")
	panic("unreachable")
}

func newEtcd(client etcdclient.Client, pathPrefix string) (etcdStorage storage.Storage, err error) {

	simpleCodec := conversion.NewSimpleCodec()
	simpleCodec.AddKnownTypes(&registry.VMTEvent{})
	simpleCodec.AddKnownTypes(&registry.VMTEventList{})
	return etcdhelper.NewEtcdStorage(client, simpleCodec, pathPrefix), nil
}