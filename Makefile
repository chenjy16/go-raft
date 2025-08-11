.PHONY: build test clean run-example fmt vet lint

# 构建项目
build:
	go build ./...

# 运行测试
test:
	go test -v ./...

# 运行测试并显示覆盖率
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# 格式化代码
fmt:
	go fmt ./...

# 静态分析
vet:
	go vet ./...

# 运行 golint（需要先安装：go install golang.org/x/lint/golint@latest）
lint:
	golint ./...

# 清理构建文件
clean:
	go clean ./...
	rm -f coverage.out coverage.html

# 运行示例程序（3节点集群）
run-example:
	@echo "Starting 3-node Raft cluster..."
	@echo "Node 1 (port 8001):"
	go run examples/simple/main.go 1 8001 8002 8003 &
	@sleep 1
	@echo "Node 2 (port 8002):"
	go run examples/simple/main.go 2 8002 8001 8003 &
	@sleep 1
	@echo "Node 3 (port 8003):"
	go run examples/simple/main.go 3 8003 8001 8002 &
	@echo "All nodes started. Press Ctrl+C to stop."
	@wait

# 运行单个节点示例
run-single:
	go run examples/simple/main.go 1 8001

# 检查代码质量
check: fmt vet test

# 安装依赖工具
install-tools:
	go install golang.org/x/lint/golint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# 初始化 go mod
mod-init:
	go mod init github.com/chenjianyu/go-raft
	go mod tidy

# 更新依赖
mod-update:
	go mod tidy
	go mod download

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  build         - Build the project"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golint"
	@echo "  clean         - Clean build files"
	@echo "  run-example   - Run 3-node cluster example"
	@echo "  run-single    - Run single node example"
	@echo "  check         - Run fmt, vet, and test"
	@echo "  install-tools - Install development tools"
	@echo "  mod-init      - Initialize go module"
	@echo "  mod-update    - Update dependencies"
	@echo "  help          - Show this help message"