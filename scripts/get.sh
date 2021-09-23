#!/usr/bin/env bash
set -euo pipefail

target="${1}"
os="$(uname -s | awk '{print tolower($0)}')"
arch="$(uname -m)"

case "${arch}" in
	aarch64) arch="arm64";;
	x86_64)  arch="amd64";;
esac

repo="eteu-technologies/amqp-deployer"
artifacts="$(
curl -s -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/"${repo}"/releases/latest \
	| jq -c -r '.assets[] | {name: .name, url: .browser_download_url}'
)"

while read -r obj; do
	name="$(jq -r '.name' <<< "${obj}")"

	binname="$(awk -F. '{print $1}' <<< "${name}")"
	[ "${binname}" = "${target}" ] || continue

	if [ "${binname}.${os}.${arch}" = "${name}" ]; then
		url="$(jq -r '.url' <<< "${obj}")"
		echo ">>> Downloading '${binname}' from '${url}'"

		curl -L -o "${binname}" "${url}"
		chmod +x "${binname}"
		exit 0
	fi
done <<< "${artifacts}"

echo ">>> Could not find artifact '${target}'"
exit 1
