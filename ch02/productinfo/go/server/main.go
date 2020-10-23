// Go to ${grpc-up-and-running}/samples/ch02/productinfo
// Optional: Execute protoc -I proto proto/product_info.proto --go_out=plugins=grpc:go/product_info
// Execute go get -v github.com/grpc-up-and-running/samples/ch02/productinfo/go/product_info
// Execute go run go/server/main.go

package main

import (
	"context"
	"log"
	"net"
	"fmt"

	"github.com/gofrs/uuid"
	proto "google.golang.org/protobuf/proto"
// 	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	grpcreflect "github.com/jhump/protoreflect/grpcreflect"
	"github.com/jhump/protoreflect/desc"
	pb "productinfo/server/ecommerce"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	port = ":50051"
)

// server is used to implement ecommerce/product_info.
type server struct {
	productMap map[string]*pb.Product
}

// AddProduct implements ecommerce.AddProduct
func (s *server) AddProduct(ctx context.Context,
							in *pb.Product) (*pb.ProductID, error) {
    Redact(in)
	out, err := uuid.NewV4()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error while generating Product ID", err)
	}
	in.Id = out.String()
	if s.productMap == nil {
		s.productMap = make(map[string]*pb.Product)
	}
	log.Printf("Product %v : %v - Added.", in.Id, in.Name)
	s.productMap[in.Id] = in
	log.Printf("Product %v : %v - Added.", in.Id, in.Name)
	return &pb.ProductID{Value: in.Id}, status.New(codes.OK, "").Err()
}

// GetProduct implements ecommerce.GetProduct
func (s *server) GetProduct(ctx context.Context, in *pb.ProductID) (*pb.Product, error) {
	product, exists := s.productMap[in.Value]
	if exists && product != nil {
		log.Printf("Product %v : %v - Retrieved.", product.Id, product.Name)
		return product, status.New(codes.OK, "").Err()
	}
	return nil, status.Errorf(codes.NotFound, "Product does not exist.", in.Value)
}

var methods = map[string]*desc.MethodDescriptor{}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	// this example just shows how to do this with a unary interceptor, but
    // the same approach works for streaming
    interceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
        log.Printf("invoked methodName: %v", info.FullMethod)
        md := methods[info.FullMethod]
        log.Printf("invoked md: %v", md.String())

        opts := md.GetMethodOptions() // we get the descriptor's options
        // and then get the custom option from that message
        log.Printf("Ops: %v", opts.String())
        nestedVal := proto.GetExtension(opts, pb.E_Throttled)
        nestedMsg, ok := nestedVal.(int32)
        if ok {
          // this will print "asdf"
          fmt.Println(nestedMsg)
        }
        // stash md in a context using context.WithValue, which now
        // now provides access to it from interceptor and handlers
        newCtx := context.WithValue(ctx, "method_descriptor", md)
        return handler(newCtx, req)
    }
    // register listener
	s := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	ser := server{}
	pb.RegisterProductInfoService(s, &pb.ProductInfoService{AddProduct: ser.AddProduct, GetProduct: ser.GetProduct})

	// when done registering services, you can get the descriptors from the
    // gRPC server and stash them into the map that the interceptor uses:
    // (error handling omitted for brevity)
    sds, _ := grpcreflect.LoadServiceDescriptors(s)
    for _, sd := range sds {
        sopts := sd.GetServiceOptions() // we get the descriptor's options
        // and then get the custom option from that message
        log.Printf("Ops: %v", sopts.String())
        val := proto.GetExtension(sopts, pb.E_Endpoint)
        ep, ok := val.(string)
        if ok {
          // this will print "asdf"
          fmt.Println(ep)
        }


        for _, md := range sd.GetMethods() {
            methodName := fmt.Sprintf("/%s/%s", sd.GetFullyQualifiedName(), md.GetName())
            log.Printf("methodName: %v", methodName)
            methods[methodName] = md
            log.Printf(methods[methodName].String())
        }
    }

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Redact clears every sensitive field in pb.
func Redact(msg proto.Message) {
	msgReflect := msg.ProtoReflect()
	msgReflect.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
	                log.Printf(v.String())
                  opts := fd.Options().(*descriptorpb.FieldOptions)
                  log.Printf(opts.String())
                  if proto.GetExtension(opts, pb.E_Sensitive).(bool) {
                      msgReflect.Clear(fd)
                      return true
                  }
                  return true
              })
}
