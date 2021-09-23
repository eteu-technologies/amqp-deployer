#!/usr/bin/env bash
set -euo pipefail

programs=( ./cmd/* )
targets=( arm64-darwin amd64-darwin arm64-linux amd64-linux )

tag="$(git describe --abbrev=0)"
version_flag="-X github.com/eteu-technologies/amqp-deployer/internal/core.Version=${tag}"

rm -rf out
mkdir -p out
for target in "${targets[@]}"; do
	for program in "${programs[@]}"; do
		GOARCH="$(awk -F- '{print $1}' <<< "${target}")"
		GOOS="$(awk -F- '{print $2}' <<< "${target}")"

		name="$(basename "${program}").${GOOS}.${GOARCH}"
		cmd=(env GOOS="${GOOS}" GOARCH="${GOARCH}" go build -ldflags "'-s -w ${version_flag}'" -trimpath -o "out/${name}" "${program}")
		echo "${cmd[@]}"
		"${cmd[@]}"
	done
done

pushd out >/dev/null
sha256sum ./* > SHA256SUMS
popd >/dev/null
