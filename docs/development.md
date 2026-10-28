# Development and releases

[Back to README](../README.md) · [Build from source](install.md#build-from-source)

## Run checks

```zsh
go test ./...
go vet ./...
go build -o bin/dia .
zsh tests/smoke.zsh
```

The smoke suite also uses Python 3 for PTY display checks. Tests use fake Dia objects and never write to real browser tabs. A live read-only check can use `dia count` or `dia profiles`. Listing reads properties in bulk and checks tab ID order before and after those reads. A detected change asks you to retry. The reads are not an atomic browser snapshot. No URLs or titles are saved to the row cache.

## Release maintenance

For each stable `vX.Y.Z` release, commit the source changes, tag that exact commit, and push the commit and tag. CI caches Go dependencies and build outputs using `go.mod`. A main-branch run skips its duplicate test matrix only when a remote stable version tag points to that same commit. Tag runs always test natively on Apple Silicon and Intel.

Each tag test job builds a versioned `bin/dia`, runs the smoke suite against that binary, checks its version, and uploads it. The release job downloads both binaries from that same workflow run and packages them without rebuilding. `go run ./cmd/release --binaries-dir dist/binaries vX.Y.Z` reads `dist/binaries/arm64/dia` and `dist/binaries/amd64/dia`, validates their Mach-O architectures, and produces the two archives, `checksums.txt`, and `dia-cli.rb`. To build both binaries locally instead, use `go run ./cmd/release vX.Y.Z`.

The release helper requires a clean tagged source tree and refuses an existing `dist/vX.Y.Z` directory. The tag workflow creates a draft GitHub Release with all four assets, then publishes it. An existing Release is not replaced.

After the Release is public, download all four assets with `gh release download vX.Y.Z --repo Jermic/dia-cli --dir release-files`, then run `cd release-files && shasum -a 256 -c checksums.txt` to verify both archives. In a checkout of the separate `Jermic/homebrew-tap` repository, replace `Formula/dia-cli.rb` with the downloaded `release-files/dia-cli.rb`, inspect the version, two architecture URLs, and their separate SHA-256 values, then commit and push that tap change. The tap is maintained separately; this repository's workflow does not write to it.
