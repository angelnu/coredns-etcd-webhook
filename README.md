# Cert-manager CoreDNS with etcd plugin webhook

This is a cert-manager webhook for ab etcd used in coredns. It is used to get Let´s encrypt certificates using CoreDNS as DNS resolver.

It is derived from [this etcd webhook](https://github.com/kleinpa/cert-manager-etcddns-webhook) and my [dnsmadeeasy webhook](https://github.com/angelnu/dnsmadeeasy-webhook)

## Deploying the webhook

Use the [angelnu helm charts](https://github.com/angelnu/helm-charts/tree/main/charts/apps/coredns-etcd-webhook)

## Building the code (local)

```bash
make
```

or if you want build and test the code:

```bash
make test
```

## Building the code (podman)

```bash
make podman-build
```

## Create a new release

Use the GitHub releases to tag a new version. The workflow should then build and upload a new version matching the tag.