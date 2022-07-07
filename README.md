Scrabble Futures Market


# protoc

To generate pb files, run this in the base directory:

```
protoc --twirp_out=rpc --go_out=rpc --go_opt=paths=source_relative --twirp_opt=paths=source_relative ./proto/market.proto
```

Make sure you have done

```
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install github.com/twitchtv/twirp/protoc-gen-twirp@latest
```
and that you have the `protoc` compiler.
