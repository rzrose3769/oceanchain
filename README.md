[![pipeline status](https://api.travis-ci.org/33cn/plugin.svg?branch=master)](https://travis-ci.org/33cn/plugin/)
[![Go Report Card](https://goreportcard.com/badge/github.com/33cn/plugin?branch=master)](https://goreportcard.com/report/github.com/33cn/plugin)


# 基于 chain33 区块链开发 框架 开发的 oceanchain公有链系统


### 编译

```
git clone https://github.com/rzrose3769/oceanchain $GOPATH/src/github.com/rzrose3769/oceanchain
cd $GOPATH/src/github.com/rzrose3769/oceanchain
go build -i -o oceanchain
go build -i -o oceanchain-cli github.com/rzrose3769/oceanchain/cli
```

### 运行
拷贝编译好的oceanchain, oceanchain-cli, ocean.toml这三个文件置于同一个文件夹下，执行：
```
./oceanchain -f ocean.toml
```
