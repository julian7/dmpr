# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

No changes so far.

## [v0.2.0] - Aug 30, 2019

### Added

* Many-to-many relationships
* More binary operators (lt/gt/le/ge)

### Known bugs

* Has many and many-to-many relations are filled incorrectly if they are referenced as
  slice of values. Slice of pointers work nicely.

## [v0.1.0] - Aug 24, 2019

### Added

* dmpr data mapper as it naturally grew from an application

[Unreleased]: https://github.com/julian7/dmpr
[v0.2.0]: https://github.com/julian7/dmpr/releases/v0.2.0
[v0.1.0]: https://github.com/julian7/dmpr/releases/v0.1.0
