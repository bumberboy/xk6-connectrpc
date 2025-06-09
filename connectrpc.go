// Package connectrpc is the root module of the k6-connectrpc extension.
package connectrpc

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/grafana/sobek"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func init() {
	modules.Register("k6/x/connectrpc", New())
}

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the ConnectRPC module for every VU.
	ModuleInstance struct {
		vu      modules.VU
		exports map[string]interface{}
		metrics *instanceMetrics
	}

	// ProtoRegistry holds the global proto definitions that can be shared across all clients
	ProtoRegistry struct {
		mu                sync.RWMutex
		methodDescriptors map[string]protoreflect.MethodDescriptor
		methodInfos       []MethodInfo
		loaded            bool
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}

	// Global proto registry shared across all VUs
	globalProtoRegistry = &ProtoRegistry{
		methodDescriptors: make(map[string]protoreflect.MethodDescriptor),
		methodInfos:       []MethodInfo{},
	}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	metrics, err := registerMetrics(vu.InitEnv().Registry)
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("failed to register ConnectRPC module metrics: %w", err))
	}

	mi := &ModuleInstance{
		vu:      vu,
		exports: make(map[string]interface{}),
		metrics: metrics,
	}

	mi.exports["Client"] = mi.NewClient
	mi.exports["loadProtos"] = mi.loadProtos
	mi.exports["loadProtoset"] = mi.loadProtoset
	mi.exports["loadEmbeddedProtoset"] = mi.loadEmbeddedProtoset
	mi.defineConstants()
	mi.exports["Stream"] = mi.stream

	return mi
}

// NewClient is the JS constructor for the ConnectRPC Client.
func (mi *ModuleInstance) NewClient(_ sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	return rt.ToValue(&Client{vu: mi.vu, metrics: mi.metrics}).ToObject(rt)
}

// loadProtos loads protocol buffer definitions from proto files into the global registry
func (mi *ModuleInstance) loadProtos(importPaths sobek.Value, filenames ...sobek.Value) ([]MethodInfo, error) {
	if mi.vu.State() != nil {
		return nil, errors.New("loadProtos must be called in the init context")
	}

	// Convert sobek values to Go types
	var importPathsSlice []string
	if importPaths != nil && !common.IsNullish(importPaths) {
		importPathsArray := importPaths.ToObject(mi.vu.Runtime())
		if importPathsArray != nil {
			for i := 0; ; i++ {
				val := importPathsArray.Get(fmt.Sprintf("%d", i))
				if val == nil || common.IsNullish(val) {
					break
				}
				importPathsSlice = append(importPathsSlice, val.String())
			}
		}
	}

	var filenamesSlice []string
	for _, filename := range filenames {
		if !common.IsNullish(filename) {
			filenamesSlice = append(filenamesSlice, filename.String())
		}
	}

	return globalProtoRegistry.loadProtos(mi.vu, importPathsSlice, filenamesSlice...)
}

// loadProtoset loads protocol buffer definitions from a protoset file into the global registry
func (mi *ModuleInstance) loadProtoset(protosetPath sobek.Value) ([]MethodInfo, error) {
	if mi.vu.State() != nil {
		return nil, errors.New("loadProtoset must be called in the init context")
	}

	if common.IsNullish(protosetPath) {
		return nil, errors.New("protosetPath cannot be null or undefined")
	}

	return globalProtoRegistry.loadProtoset(mi.vu, protosetPath.String())
}

// loadEmbeddedProtoset loads protocol buffer definitions from base64-encoded protoset data into the global registry
func (mi *ModuleInstance) loadEmbeddedProtoset(protosetData sobek.Value) ([]MethodInfo, error) {
	if common.IsNullish(protosetData) {
		return nil, errors.New("protosetData cannot be null or undefined")
	}

	return globalProtoRegistry.loadEmbeddedProtoset(protosetData.String())
}

// defineConstants defines the constant variables of the module.
func (mi *ModuleInstance) defineConstants() {
	rt := mi.vu.Runtime()

	// Protocol constants
	mi.exports["PROTOCOL_CONNECT"] = rt.ToValue("connect")
	mi.exports["PROTOCOL_GRPC"] = rt.ToValue("grpc")
	mi.exports["PROTOCOL_GRPC_WEB"] = rt.ToValue("grpc-web")

	// Content type constants
	mi.exports["CONTENT_TYPE_JSON"] = rt.ToValue("application/json")
	mi.exports["CONTENT_TYPE_PROTOBUF"] = rt.ToValue("application/protobuf")
	mi.exports["CONTENT_TYPE_GRPC"] = rt.ToValue("application/grpc")
	mi.exports["CONTENT_TYPE_GRPC_WEB"] = rt.ToValue("application/grpc-web")

	// HTTP status codes for Connect protocol
	mi.exports["StatusOK"] = rt.ToValue(200)
	mi.exports["StatusBadRequest"] = rt.ToValue(400)
	mi.exports["StatusUnauthorized"] = rt.ToValue(401)
	mi.exports["StatusForbidden"] = rt.ToValue(403)
	mi.exports["StatusNotFound"] = rt.ToValue(404)
	mi.exports["StatusInternalServerError"] = rt.ToValue(500)
	mi.exports["StatusNotImplemented"] = rt.ToValue(501)
	mi.exports["StatusServiceUnavailable"] = rt.ToValue(503)
}

// Exports returns the exports of the connectrpc module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: mi.exports,
	}
}

// stream returns a new stream object
func (mi *ModuleInstance) stream(c sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()

	client, err := extractClient(c.Argument(0), rt)
	if err != nil {
		common.Throw(rt, fmt.Errorf("invalid ConnectRPC Stream's client: %w", err))
	}

	methodName := sanitizeMethodName(c.Argument(1).String())
	methodDescriptor, err := client.getMethodDescriptor(methodName)
	if err != nil {
		common.Throw(rt, fmt.Errorf("invalid ConnectRPC Stream's method: %w", err))
	}

	p, err := newCallParams(mi.vu, c.Argument(2))
	if err != nil {
		common.Throw(rt, fmt.Errorf("invalid ConnectRPC Stream's parameters: %w", err))
	}

	p.SetSystemTags(mi.vu.State(), client.addr, methodName)

	logger := mi.vu.State().Logger.WithField("streamMethod", methodName)

	s := &stream{
		vu:               mi.vu,
		client:           client,
		methodDescriptor: methodDescriptor,
		method:           methodName,
		logger:           logger,

		tq: taskqueue.New(mi.vu.RegisterCallback),

		instanceMetrics: mi.metrics,
		builtinMetrics:  mi.vu.State().BuiltinMetrics,
		done:            make(chan struct{}),
		writingState:    opened,

		writeQueueCh: make(chan message),

		eventListeners: newEventListeners(),
		obj:            rt.NewObject(),
		tagsAndMeta:    &p.TagsAndMeta,
	}

	defineStream(rt, s)

	err = s.beginStream(p)
	if err != nil {
		s.tq.Close()
		common.Throw(rt, err)
	}

	return s.obj
}

// extractClient extracts & validates a connectrpc.Client from a sobek.Value.
func extractClient(v sobek.Value, rt *sobek.Runtime) (*Client, error) {
	if common.IsNullish(v) {
		return nil, errors.New("empty ConnectRPC client")
	}

	client, ok := v.ToObject(rt).Export().(*Client)
	if !ok {
		return nil, errors.New("not a ConnectRPC client")
	}

	// For per-call strategy, we need connectParams instead of httpClient
	if client.connectionStrategy == "per-call" {
		if client.connectParams == nil {
			return nil, errors.New("no ConnectRPC connection parameters, you must call connect first")
		}
	} else {
		// For per-vu and per-iteration strategies, we need httpClient
		if client.httpClient == nil {
			return nil, errors.New("no ConnectRPC connection, you must call connect first")
		}
	}

	return client, nil
}

// sanitizeMethodName ensures the method name has the correct format
func sanitizeMethodName(name string) string {
	if name == "" {
		return name
	}

	if name[0] != '/' {
		name = "/" + name
	}

	return name
}

// loadProtos loads protocol buffer definitions from proto files into the global registry
func (registry *ProtoRegistry) loadProtos(vu modules.VU, importPaths []string, filenames ...string) ([]MethodInfo, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	// Use the same loading logic as Client.Load but store in global registry
	initEnv := vu.InitEnv()
	if initEnv == nil {
		return nil, errors.New("missing init environment")
	}

	// If no import paths are specified, use the current working directory
	if len(importPaths) == 0 {
		importPaths = append(importPaths, initEnv.CWD.Path)
	}

	for i, s := range importPaths {
		// Clean file scheme as it is the only supported scheme and the following APIs do not support them
		importPaths[i] = strings.TrimPrefix(s, "file://")
	}

	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: false,
		Accessor: protoparse.FileAccessor(func(filename string) (io.ReadCloser, error) {
			absFilePath := initEnv.GetAbsFilePath(filename)
			return initEnv.FileSystems["file"].Open(absFilePath)
		}),
	}

	fds, err := parser.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	fdset := &descriptorpb.FileDescriptorSet{}
	seen := make(map[string]struct{})
	for _, fd := range fds {
		fdset.File = append(fdset.File, walkFileDescriptors(seen, fd)...)
	}

	methods, err := registry.convertToMethodInfo(fdset)
	if err != nil {
		return nil, err
	}

	registry.methodInfos = append(registry.methodInfos, methods...)
	registry.loaded = true

	return methods, nil
}

// loadProtoset loads protocol buffer definitions from a protoset file into the global registry
func (registry *ProtoRegistry) loadProtoset(vu modules.VU, protosetPath string) ([]MethodInfo, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	// Use the same loading logic as Client.LoadProtoset but store in global registry
	initEnv := vu.InitEnv()
	if initEnv == nil {
		return nil, errors.New("missing init environment")
	}

	absFilePath := initEnv.GetAbsFilePath(protosetPath)
	fdsetFile, err := initEnv.FileSystems["file"].Open(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open protoset: %w", err)
	}

	defer func() { _ = fdsetFile.Close() }()
	fdsetBytes, err := io.ReadAll(fdsetFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't read protoset: %w", err)
	}

	fdset := &descriptorpb.FileDescriptorSet{}
	if err = proto.Unmarshal(fdsetBytes, fdset); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal protoset file %s: %w", protosetPath, err)
	}

	methods, err := registry.convertToMethodInfo(fdset)
	if err != nil {
		return nil, err
	}

	registry.methodInfos = append(registry.methodInfos, methods...)
	registry.loaded = true

	return methods, nil
}

// loadEmbeddedProtoset loads protocol buffer definitions from base64-encoded protoset data into the global registry
func (registry *ProtoRegistry) loadEmbeddedProtoset(base64Data string) ([]MethodInfo, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	// Decode base64 data
	fdsetBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode base64 protoset data: %w", err)
	}

	fdset := &descriptorpb.FileDescriptorSet{}
	if err = proto.Unmarshal(fdsetBytes, fdset); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal embedded protoset: %w", err)
	}

	methods, err := registry.convertToMethodInfo(fdset)
	if err != nil {
		return nil, err
	}

	registry.methodInfos = append(registry.methodInfos, methods...)
	registry.loaded = true

	return methods, nil
}

// convertToMethodInfo converts a FileDescriptorSet to MethodInfo and stores descriptors in the registry
func (registry *ProtoRegistry) convertToMethodInfo(fdset *descriptorpb.FileDescriptorSet) ([]MethodInfo, error) {
	files, err := protodesc.NewFiles(fdset)
	if err != nil {
		return nil, err
	}
	var rtn []MethodInfo

	appendMethodInfo := func(
		fd protoreflect.FileDescriptor,
		sd protoreflect.ServiceDescriptor,
		md protoreflect.MethodDescriptor,
	) {
		name := fmt.Sprintf("/%s/%s", sd.FullName(), md.Name())
		registry.methodDescriptors[name] = md
		rtn = append(rtn, MethodInfo{
			Package:        string(fd.Package()),
			Service:        string(sd.Name()),
			FullMethod:     name,
			IsClientStream: md.IsStreamingClient(),
			IsServerStream: md.IsStreamingServer(),
		})
	}

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			sd := services.Get(i)
			methods := sd.Methods()
			for j := 0; j < methods.Len(); j++ {
				md := methods.Get(j)
				appendMethodInfo(fd, sd, md)
			}
		}
		return true
	})

	return rtn, nil
}

// getMethodDescriptor gets a method descriptor from the global registry
func (registry *ProtoRegistry) getMethodDescriptor(method string) (protoreflect.MethodDescriptor, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	if !registry.loaded {
		return nil, errors.New("no proto files loaded: call loadProtos() or loadProtoset() first")
	}

	method = sanitizeMethodName(method)
	methodDesc := registry.methodDescriptors[method]
	if methodDesc == nil {
		return nil, fmt.Errorf("method %q not found in loaded proto files", method)
	}

	return methodDesc, nil
}
