# Changelog

## [1.3.1](https://github.com/pacorreia/go-rdp-server/compare/v1.3.0...v1.3.1) (2026-06-01)


### Bug Fixes

* Merge pull request [#25](https://github.com/pacorreia/go-rdp-server/issues/25) from pacorreia/copilot/fix-runtime-error-login ([f59b92d](https://github.com/pacorreia/go-rdp-server/commit/f59b92df80c23d0f868cde86745f6ef120ed88df))

## [1.3.0](https://github.com/pacorreia/go-rdp-server/compare/v1.2.0...v1.3.0) (2026-05-31)


### Features

* add workflow_dispatch trigger for on-demand releases ([d5333c7](https://github.com/pacorreia/go-rdp-server/commit/d5333c75e007b4482135d6e45b754a03cabc8f41))


### Bug Fixes

* patch grdp v0.8.6 assembly - rename Intel mnemonics to Go Plan 9 equivalents ([e28f8c5](https://github.com/pacorreia/go-rdp-server/commit/e28f8c5c1431c42bb60fa12b4bfdd1c88b71202e))
* resolve merge conflicts - incorporate gorilla/websocket v1.5.3 and update Go 1.26 across CI, README, docs ([604c0a3](https://github.com/pacorreia/go-rdp-server/commit/604c0a3ded96aa0e7092d19155807206ea33d2ba))

## [1.2.0](https://github.com/pacorreia/go-rdp-server/compare/v1.1.0...v1.2.0) (2026-05-31)


### Features

* make per-user-login the default mode ([22e6f7b](https://github.com/pacorreia/go-rdp-server/commit/22e6f7b9bfca84982f112ecd97087cad9272a663))
* per-user login form with passwordless Windows account support ([611c971](https://github.com/pacorreia/go-rdp-server/commit/611c971b70f7bd2cca5aac8cb899a21992e4acaf))


### Bug Fixes

* address code review comments (Enter key dedup, close code comment, log cleanup error) ([2225a49](https://github.com/pacorreia/go-rdp-server/commit/2225a49c172764db9ca1529b3ac1dec45902586f))
* gate empty-password passwordless workaround behind --allow-passwordless flag and document new flags ([2c81494](https://github.com/pacorreia/go-rdp-server/commit/2c81494a1168108288d6e79ee80d3b4cc32c4f93))
* harden per-user login, add --allow-passwordless flag, and document new configuration options ([f2eddb8](https://github.com/pacorreia/go-rdp-server/commit/f2eddb84b247edf3d226718ca77b996d05576f5c))
* ipTracker.release use == 1 to avoid masking double-release bugs ([0c65fb2](https://github.com/pacorreia/go-rdp-server/commit/0c65fb2e25ce80170ae88dbf5706fe47f946d5b4))
* proper WebSocket close frames and static RDP credential support ([07be282](https://github.com/pacorreia/go-rdp-server/commit/07be282b69e96f38284f1de302b1f83351314aa8))

## [1.1.0](https://github.com/pacorreia/go-rdp-server/compare/v1.0.1...v1.1.0) (2026-05-31)


### Features

* add CLI flags, log-level, and service install/uninstall ([e1de410](https://github.com/pacorreia/go-rdp-server/commit/e1de4108cc5e1a58a4fb46a1f016743bbb7da491))
* replace guacd with pure-Go RDP client (nakagami/grdp) and canvas UI ([1fbc346](https://github.com/pacorreia/go-rdp-server/commit/1fbc346df1841f4b82a2185f0b984ce7096d6d7c))


### Bug Fixes

* address code review feedback on MouseWheel interface and wheel comment ([e6c83d4](https://github.com/pacorreia/go-rdp-server/commit/e6c83d4af6875d8dc55ee92c48e5ff6e2b99db97))
* align canvas to RDP resolution and guarantee tiles channel closure ([583edd9](https://github.com/pacorreia/go-rdp-server/commit/583edd97e07f0c3baf477bbf3964edef3b145a02))
* close tiles channel in closeDone to unblock tileLoop on disconnect ([1e41c2d](https://github.com/pacorreia/go-rdp-server/commit/1e41c2d47bc437e68321f33f569ff09cebb18390))
* close tiles channel on RDP disconnect to unblock tileLoop ([9842960](https://github.com/pacorreia/go-rdp-server/commit/9842960b266c7f48f056052f7849c3abd2cb9460))
* move close(s.done) inside mutex lock for atomic state transition ([34b54cc](https://github.com/pacorreia/go-rdp-server/commit/34b54cc474195ed67327e862f3ce3b7d8b9ba81d))
* protect tile sends with mutex to prevent send-on-closed-channel race ([1cce430](https://github.com/pacorreia/go-rdp-server/commit/1cce43095b0d742200ca984eb277114441c3c8ba))

## [1.0.1](https://github.com/pacorreia/go-rdp-server/compare/v1.0.0...v1.0.1) (2026-05-31)


### Bug Fixes

* align docs theme palette with vaults-syncer ([fa432e3](https://github.com/pacorreia/go-rdp-server/commit/fa432e3dd55005c111b35cd2f7cbd386f1e1f6cf))
* align palette with vaults-syncer - add media queries and material icons ([cadb4bc](https://github.com/pacorreia/go-rdp-server/commit/cadb4bc68d099e6362e8ef8f52710985bbb25009))

## 1.0.0 (2026-05-31)


### Features

* add windows service runtime support ([f3a10b3](https://github.com/pacorreia/go-rdp-server/commit/f3a10b3ff4becb79c0d6cde272f5b05b0263431c))
* migrate docs to MkDocs Material theme (matching vaults-syncer) ([146cc80](https://github.com/pacorreia/go-rdp-server/commit/146cc80e7b7fff8a3ef4702098641413eb1e1ed4))
* switch docs from MkDocs to Zensical ([c46b6f3](https://github.com/pacorreia/go-rdp-server/commit/c46b6f3291199188ea191c1793e9b87fdaee80a8))


### Bug Fixes

* address review comments in test and windows error handling ([497a014](https://github.com/pacorreia/go-rdp-server/commit/497a014b0db8f3987f84f8055e01fd53806fc969))
* fix Mermaid rendering by calling mermaid.run() after DOM replacement ([4386eb0](https://github.com/pacorreia/go-rdp-server/commit/4386eb05431cf78ffc2ba0546131fd9c67bb7e02))
* support windows service lifecycle ([83c2bb0](https://github.com/pacorreia/go-rdp-server/commit/83c2bb0dcad9866fbb08d8c3c4160edc5e22abcd))


### Reverts

* restore peaceiris/actions-gh-pages strategy in docs-pages workflow ([d235997](https://github.com/pacorreia/go-rdp-server/commit/d2359972fd3811a5d94ba19c6677784ae4fa1a9d))
