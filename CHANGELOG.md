# Changelog

## v0.1.0-rc.1 (2026-10-23)

### Features

- **config:** add a tag-prefix per component ([#30](https://github.com/ostendorf-cloud/release-me/pull/30))
- **ci:** rerun the action when a release pull request is labeled ([#27](https://github.com/ostendorf-cloud/release-me/pull/27))
- **version:** versioning modes for components, independent, locked or linked ([#25](https://github.com/ostendorf-cloud/release-me/pull/25))
- **changelog:** configurable source, first-parent, squash, all or nested ([#24](https://github.com/ostendorf-cloud/release-me/pull/24))
- **releasepr:** explain how the release works in the pull request body ([#23](https://github.com/ostendorf-cloud/release-me/pull/23))
- **action:** attribute the release commit to the app bot account ([#18](https://github.com/ostendorf-cloud/release-me/pull/18))
- composite action, image, binaries and self release ([#14](https://github.com/ostendorf-cloud/release-me/pull/14))
- typed updaters and the moving major tag ([#13](https://github.com/ostendorf-cloud/release-me/pull/13))
- pre-releases through labels on the release pull request ([#12](https://github.com/ostendorf-cloud/release-me/pull/12))
- components with their own tags and release pull requests ([#11](https://github.com/ostendorf-cloud/release-me/pull/11))
- release pull request, tag and release for one component ([#9](https://github.com/ostendorf-cloud/release-me/pull/9))
- next and changelog from the first parent history ([#8](https://github.com/ostendorf-cloud/release-me/pull/8))
- **cli:** command skeleton with usage and version

### Bug Fixes

- **pr:** a pre-release label change pushes the release branch ([#31](https://github.com/ostendorf-cloud/release-me/pull/31))
- **pr:** commit the release branch with an explicit git identity ([#16](https://github.com/ostendorf-cloud/release-me/pull/16))
