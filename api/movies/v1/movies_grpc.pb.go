package moviesv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion9

const (
	MovieService_LoadMovies_FullMethodName   = "/movies.v1.MovieService/LoadMovies"
	MovieService_GetLastMovie_FullMethodName = "/movies.v1.MovieService/GetLastMovie"
)

type MovieServiceClient interface {
	LoadMovies(ctx context.Context, in *LoadMoviesRequest, opts ...grpc.CallOption) (*LoadMoviesResponse, error)
	GetLastMovie(ctx context.Context, in *GetLastMovieRequest, opts ...grpc.CallOption) (*Movie, error)
}

type movieServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewMovieServiceClient(cc grpc.ClientConnInterface) MovieServiceClient {
	return &movieServiceClient{cc}
}

func (c *movieServiceClient) LoadMovies(ctx context.Context, in *LoadMoviesRequest, opts ...grpc.CallOption) (*LoadMoviesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LoadMoviesResponse)
	err := c.cc.Invoke(ctx, MovieService_LoadMovies_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *movieServiceClient) GetLastMovie(ctx context.Context, in *GetLastMovieRequest, opts ...grpc.CallOption) (*Movie, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Movie)
	err := c.cc.Invoke(ctx, MovieService_GetLastMovie_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type MovieServiceServer interface {
	LoadMovies(context.Context, *LoadMoviesRequest) (*LoadMoviesResponse, error)
	GetLastMovie(context.Context, *GetLastMovieRequest) (*Movie, error)
	mustEmbedUnimplementedMovieServiceServer()
}

type UnimplementedMovieServiceServer struct{}

func (UnimplementedMovieServiceServer) LoadMovies(context.Context, *LoadMoviesRequest) (*LoadMoviesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method LoadMovies not implemented")
}
func (UnimplementedMovieServiceServer) GetLastMovie(context.Context, *GetLastMovieRequest) (*Movie, error) {
	return nil, status.Error(codes.Unimplemented, "method GetLastMovie not implemented")
}
func (UnimplementedMovieServiceServer) mustEmbedUnimplementedMovieServiceServer() {}
func (UnimplementedMovieServiceServer) testEmbeddedByValue()                      {}

type UnsafeMovieServiceServer interface {
	mustEmbedUnimplementedMovieServiceServer()
}

func RegisterMovieServiceServer(s grpc.ServiceRegistrar, srv MovieServiceServer) {

	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&MovieService_ServiceDesc, srv)
}

func _MovieService_LoadMovies_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoadMoviesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MovieServiceServer).LoadMovies(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MovieService_LoadMovies_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MovieServiceServer).LoadMovies(ctx, req.(*LoadMoviesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MovieService_GetLastMovie_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetLastMovieRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MovieServiceServer).GetLastMovie(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MovieService_GetLastMovie_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MovieServiceServer).GetLastMovie(ctx, req.(*GetLastMovieRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var MovieService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "movies.v1.MovieService",
	HandlerType: (*MovieServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "LoadMovies",
			Handler:    _MovieService_LoadMovies_Handler,
		},
		{
			MethodName: "GetLastMovie",
			Handler:    _MovieService_GetLastMovie_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/movies/v1/movies.proto",
}
