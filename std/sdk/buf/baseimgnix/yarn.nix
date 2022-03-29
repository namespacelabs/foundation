{
  fetchurl,
  fetchgit,
  linkFarm,
  runCommand,
  gnutar,
}: rec {
  offline_cache = linkFarm "offline" packages;
  packages = [
    {
      name = "google_protobuf___google_protobuf_3.19.4.tgz";
      path = fetchurl {
        name = "google_protobuf___google_protobuf_3.19.4.tgz";
        url = "https://registry.yarnpkg.com/google-protobuf/-/google-protobuf-3.19.4.tgz";
        sha512 = "OIPNCxsG2lkIvf+P5FNfJ/Km95CsXOBecS9ZcAU6m2Rq3svc0Apl9nB3GMDNKfQ9asNv4KjyAqGwPQFrVle3Yg==";
      };
    }
    {
      name = "ts_protoc_gen___ts_protoc_gen_0.15.0.tgz";
      path = fetchurl {
        name = "ts_protoc_gen___ts_protoc_gen_0.15.0.tgz";
        url = "https://registry.yarnpkg.com/ts-protoc-gen/-/ts-protoc-gen-0.15.0.tgz";
        sha512 = "TycnzEyrdVDlATJ3bWFTtra3SCiEP0W0vySXReAuEygXCUr1j2uaVyL0DhzjwuUdQoW5oXPwk6oZWeA0955V+g==";
      };
    }
  ];
}
