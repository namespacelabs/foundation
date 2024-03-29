# Namespace Library

This directory holds a set of resources and extensions that are supported and
maintained by Namespace Labs.

These resources can be referred to by using the
`library.namespace.so/{relative}` form, where `{relative}` would be relative
path under this directory.

The presence of particular resources or extensions in this directory do not
indicate that Namespace Labs endorses the use of some infrastructure products,
vs others. Our goal is to support our users where they are.

The Namespace platform is built so that anyone can build resource providers and
extensions of their own, and we'd like to support the community's effort in
doing so.

## Directory Structure

Resource classes are grouped by their purpose (e.g. `storage/s3`, `database/postgres`).

Resource providers are grouped by their provider (e.g. `aws/rds`), with open-source providers grouped under `oss`.
