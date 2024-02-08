/*
  SPDX-License-Identifier: MIT-0
  Copyright (c) 2024 Lars Ekman, uablrek@gmail.com
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	deviceapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var version = "unknown"

type plugin struct {
	deviceapi.DevicePluginServer
	count    int
	socket   string
	resource string
	logger   logr.Logger
}

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	lvl := flag.Int("loglevel", 0, "Log level")
	socket := flag.String("socket", "token-resource", "Unix socket for the grpc server")
	resource := flag.String("resource", "example.com/token", "The resource name")
	count := flag.Int("count", 1, "Number of resources")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	logger := createLogger(*lvl)
	logger.Info("Start", "version", version, "resource", *resource, "count", *count)
	if *count < 1 {
		logger.Error(
			fmt.Errorf("Value to low"), "Resource count", "count", *count)
		return
	}

	ctx, _ := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM)

	p := &plugin{
		count:    *count,
		socket:   filepath.Join(deviceapi.DevicePluginPath, *socket),
		resource: *resource,
		logger:   logger,
	}

	for ctx.Err() == nil {
		// Listen must work. Exit immediately if it's not
		// (e.g. permission denied).
		_ = os.Remove(p.socket) // There may be an old file lingering
		l, err := net.Listen("unix", p.socket)
		if err != nil {
			logger.Error(err, "Listen")
			return
		}

		// We must be able to cancel any operation if kubelet restarts
		// and deletes our listen socket, but the program should
		// continue so we can't cancel the passed context (ctx).
		ctxc, cancel := context.WithCancel(ctx)

		// Errors on this channel are:
		//  1. The gRPC server fails
		//  2. The socket is removed (kubelet restart)
		ch := make(chan error)

		go func() {
			ch <- monitorSocket(ctxc, p.socket)
		}()
		go func() {
			ch <- p.serve(ctxc, l)
		}()
		go p.register(ctxc)

		select {
		case <-ctx.Done():
			// ctx.Err() is non-nil, so quit the normal way
		case err = <-ch:
			cancel()
			<-ch // Wait for the error from the "other" component
			close(ch)
			_ = l.Close() // May be closed already
			logger.Error(err, "Will try again")
		}
	}
	logger.Error(ctx.Err(), "Quitting")
}

// serve Start a DevicePluginServer
func (p *plugin) serve(ctx context.Context, l net.Listener) error {
	grpcServer := grpc.NewServer()
	deviceapi.RegisterDevicePluginServer(grpcServer, p)
	ch := make(chan error)
	go func() {
		ch <- grpcServer.Serve(l)
	}()
	var err error
	select {
	case err = <-ch:
	case <-ctx.Done():
		grpcServer.Stop()
		<-ch // Wait until the grpcServer stops
	}
	close(ch)
	return err
}

// register Register to kubelet. This function will re-try until success
// or the context is cancelled.
func (p *plugin) register(ctx context.Context) {
	var delay time.Duration
	for ctx.Err() == nil {
		if err := sleep(ctx, delay); err != nil {
			return
		}
		delay = time.Second
		conn, err := grpc.DialContext(
			ctx, "unix://"+deviceapi.KubeletSocket,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		if err != nil {
			p.logger.Error(err, "Dial to KubeletSocket")
			continue
		}
		defer conn.Close()
		p.logger.Info("Connected to KubeletSocket")

		client := deviceapi.NewRegistrationClient(conn)
		request := &deviceapi.RegisterRequest{
			Version:      deviceapi.Version,
			Endpoint:     p.socket,
			ResourceName: p.resource,
		}
		if _, err = client.Register(ctx, request); err != nil {
			p.logger.Error(err, "Register to Kubelet")
			continue
		}
		p.logger.Info("Registered to Kubelet")
		break
	}
}

// monitorSocket Returns an error if the socked disappears, or if the
// context is cancelled.
func monitorSocket(ctx context.Context, socket string) error {
	for {
		if _, err := os.Lstat(socket); err != nil {
			return err
		}
		if err := sleep(ctx, time.Second*2); err != nil {
			return err
		}
	}
}

// The DevicePluginServer interface:

func (p *plugin) GetDevicePluginOptions(
	context.Context, *deviceapi.Empty) (*deviceapi.DevicePluginOptions, error) {
	p.logger.V(1).Info("GetDevicePluginOptions")
	return &deviceapi.DevicePluginOptions{}, nil
}
func (p *plugin) ListAndWatch(
	_ *deviceapi.Empty, stream deviceapi.DevicePlugin_ListAndWatchServer) error {
	res := new(deviceapi.ListAndWatchResponse)
	for i := 0; i < p.count; i++ {
		res.Devices = append(res.Devices, &deviceapi.Device{
			ID: fmt.Sprintf("item-%d", i), Health: deviceapi.Healthy})
	}
	p.logger.Info("ListAndWatch", "response", *res)
	if err := stream.Send(res); err != nil {
		p.logger.Error(err, "Send responce")
		return err
	}

	p.logger.V(1).Info("ListAndWatch waiting...")
	select {}
}
func (p *plugin) Allocate(
	_ context.Context, req *deviceapi.AllocateRequest) (*deviceapi.AllocateResponse, error) {
	p.logger.V(1).Info("Allocate", "AllocateRequest", *req)
	res := &deviceapi.AllocateResponse{
		ContainerResponses: make(
			[]*deviceapi.ContainerAllocateResponse, 0, len(req.ContainerRequests)),
	}
	for range req.ContainerRequests {
		resp := new(deviceapi.ContainerAllocateResponse)
		res.ContainerResponses = append(res.ContainerResponses, resp)
	}
	return res, nil
}
func (p *plugin) PreStartContainer(
	context.Context, *deviceapi.PreStartContainerRequest) (
	*deviceapi.PreStartContainerResponse, error) {
	return &deviceapi.PreStartContainerResponse{}, nil
}
func (p *plugin) GetPreferredAllocation(
	context.Context, *deviceapi.PreferredAllocationRequest) (
	*deviceapi.PreferredAllocationResponse, error) {
	return &deviceapi.PreferredAllocationResponse{}, nil
}

// sleep Like time.Sleep but with a context
func sleep(ctx context.Context, t time.Duration) error {
	if t == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(t):
	}
	return nil
}

// createLogger Create a Zap logger
func createLogger(lvl int) logr.Logger {
	zc := zap.NewProductionConfig()
	zc.Level = zap.NewAtomicLevelAt(zapcore.Level(-lvl))
	zc.DisableStacktrace = true
	zc.DisableCaller = true
	zc.Sampling = nil
	zc.EncoderConfig.TimeKey = "time"
	zc.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	//zc.OutputPaths = []string{"stdout"}
	z, err := zc.Build()
	if err != nil {
		panic(fmt.Sprintf("Can't create a zap logger (%v)?", err))
	}
	return zapr.NewLogger(z)
}
