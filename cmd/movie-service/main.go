package main

import (
	"context"
	"errors"
	"fmt"
	pb "go_hw/api/movies/v1"
	"go_hw/internal/config"
	"go_hw/internal/grpcapi"
	"go_hw/internal/httpapi"
	"go_hw/internal/movies"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func serve(ctx context.Context) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	httpListener, err := net.Listen("tcp", c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("HTTP listen: %w", err)
	}
	defer httpListener.Close()
	grpcListener, err := net.Listen("tcp", c.GRPCAddr)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}
	defer grpcListener.Close()
	client := &http.Client{}
	defer client.CloseIdleConnections()
	service := movies.New(c.BaseURL, client, 5*time.Second)
	httpServer := &http.Server{Handler: httpapi.New(service), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	grpcServer := grpc.NewServer()
	pb.RegisterMovieServiceServer(grpcServer, &grpcapi.Server{Service: service})
	reflection.Register(grpcServer)
	errs := make(chan error, 2)
	go func() { errs <- httpServer.Serve(httpListener) }()
	go func() { errs <- grpcServer.Serve(grpcListener) }()
	log.Printf("HTTP %s; gRPC %s", httpListener.Addr(), grpcListener.Addr())
	select {
	case <-ctx.Done():
	case err = <-errs:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(done) }()
	if e := httpServer.Shutdown(shutdown); e != nil {
		httpServer.Close()
	}
	select {
	case <-done:
	case <-shutdown.Done():
		grpcServer.Stop()
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := serve(ctx)
	stop()
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
