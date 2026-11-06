#!/bin/bash

set -e -o pipefail

if [ ! -d .git ]; then
    echo 'This script must be run from repository root' 1>&2
    echo 'Usage: bash ./scripts/bump-version.bash 1.2.3' 1>&2
    exit 1
fi

if ! git diff --quiet; then
    echo 'Working tree is dirty! Ensure all changes are committed and working tree is clean' >&2
    exit 1
fi

if ! git diff --cached --quiet; then
    echo 'Git index is dirty! Ensure all changes are committed and Git index is clean' >&2
    exit 1
fi


version="$1"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "The first argument must match to '^\d+\.\d+\.\d+$': ${version}" 1>&2
    echo 'Usage: bash ./scripts/bump-version.bash 1.2.3' 1>&2
    exit 1
fi

function sed_() {
    case "$OSTYPE" in
        darwin*)
            /usr/bin/sed -i '' -E "$@"
            ;;
        *)
            sed -i -E "$@"
            ;;
    esac
}

pre_commit_hook='./.pre-commit-hooks.yaml'
usage_doc='./docs/usage.md'
tag="v${version}"
job_url='https://github.com/dhawton/actionlint/actions/workflows/release.yaml'
download_script='./scripts/download-actionlint.bash'
readme_doc='./README.md'
man_ronn='./man/actionlint.1.ronn'

echo "Bumping up version to ${version} (tag: ${tag})"

# Update container image tag in pre-commit hook (See #116 for more details)
echo "Updating $pre_commit_hook"
sed_ "s/entry: ghcr\\.io\\/dhawton\\/actionlint:.*/entry: ghcr.io\\/dhawton\\/actionlint:${version}/" "$pre_commit_hook"

echo "Updating $usage_doc"
sed_ "\
    s/    rev: v[0-9]+\.[0-9]+\.[0-9]+/    rev: v${version}/; \
    s/\`actionlint:[0-9]+\.[0-9]+\.[0-9]+\`/\`actionlint:${version}\`/g; \
    " "$usage_doc"

echo "Updating $download_script"
sed_ "s/^version=\"[0-9]+\.[0-9]+\.[0-9]+\"/version=\"${version}\"/" "$download_script"

for f in "$readme_doc" "$man_ronn"; do
    echo "Updating document links in $f"
    sed_ "s/\/dhawton\/actionlint\/blob\/v[0-9]+\.[0-9]+\.[0-9]+\/docs\//\/dhawton\/actionlint\/blob\/v${version}\/docs\//g" "$f"
done

echo "Done. Open a PR with these changes. After it merges, tag the merge commit and push the tag:"
echo "  git tag ${tag} <merge commit> && git push origin ${tag}"
echo "Release job: ${job_url}"

touch .bumptimestamp
