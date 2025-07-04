<p>
  <a href="https://namespace.so">
    <img src="https://storage.googleapis.com/namespacelabs-docs-assets/gh/banner.svg" height="100">
  </a>
</p>

<p>
  <b><i>Namespace</i> is a development-optimized compute platform. It improves the performance and observability of Docker builds, GitHub Actions, and more, without requiring workflow changes. Learn more at https://namespace.so.</b>

Namespace is a purpose-built ephemeral compute platform that is optimized for developer use-cases: high performance workloads, with high I/O requirements, where caches and incrementally are first class. Many teams use the infrastructure programmatically (via our CLI and APIs) to build custom Previews, Developer Environments, and more. Learn more about our [APIs](https://buf.build/namespace/cloud).

</p>

<p>
  This repository includes <b>Foundation</b>, the underlying technology on which Namespace's services are built. It's a composable system that drives, build, test and deployment, from a single set of service and extension definitions in CUE. It's inspired by Boq, Google's application platform, which our team also helped build.
</p>

<p>

</p>

<div>
 🎬 <a href="https://namespace.so/docs/solutions/github-actions?utm_source=github"><b>Getting Started</b></a>
 <span>&nbsp;•&nbsp;</span>
 🗼 <a href="https://namespace.so/docs?utm_source=github">Documentation</a>
 <span>&nbsp;•&nbsp;</span>
 💬 <a href="https://community.namespace.so/discord">Discord</a>
</div>

### **About Foundation**

Namespace's foundation is an application development platform that helps you manage your development, testing,
and production workflows, in a consistent and unified way.

You describe the servers in your application, how they're built, their relationship, and which
additional resources they need. And from that description -- built out of a set of composable and
extensible blocks -- Namespace orchestrates:

- **Build**: start with your Dockerfiles, or use one of our language-specific integrations, so you
  don't have to manage Dockerfiles manually. Apart from their ease of use, language-specific
  integrations set up your build and development environment with the latest best practices and make
  harder things simple (e.g., support multiple platforms).

- **Development environment**: With Namespace, you don't need to manage your development
  dependencies manually -- the days of asking folks in the team to install the right SDK versions
  will be gone. Instead, we'll manage SDKs and dependencies on your behalf. As a result, getting
  someone new onboarded into the application development environment will take minutes rather than
  hours or days.

- **Representative environments**: How often have you hit bugs in production or testing that are
  hard to reproduce locally? Namespace helps you bridge the gap between environments, managing
  production-like environments across development and testing.

- **Effortless end-to-end testing**: When you have an application setup with Namespace, writing an
  end-to-end system test becomes simple. From the same application definition used to set up a
  development environment, Namespace is also capable of creating ephemeral testing environments used
  to run end-to-end tests.

- **Kubernetes**: (but you don't need to care). We help you and your team to think about concepts
  you care about: services, resources, backends, etc. Under the covers, you'll find
  industry-standard Kubernetes and CNCF projects, which means that as you grow or need, you can
  easily tap into the broad Kubernetes ecosystem. No hidden implementation, and no vendor lock-in.

- **Packaged dependencies**: adding support infrastructure or a resource -- whether it's a storage
  bucket, a database, a messaging system, etc. -- it's as simple as adding a dependency to your
  application.

### **Getting Started**

To get started follow our [examples](https://namespacelabs.dev/examples) our team put
together.

### **Issues**

The Namespace Labs team uses Linear for issue management. Unfortunately, we haven’t yet found a way
to make it available to everyone. Meanwhile, please file issues in Github, and we’ll follow up on
them.

### **Useful links**

- [Contributing to Namespace](/CONTRIBUTING.md)
