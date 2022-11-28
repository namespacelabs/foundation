// XXX this package should be a web node instead.
binary: {
	name: "webui"
	from: nodejs_build: {
		pkg:          "."
		node_pkg_mgr: 4 // YARN3
		prod: {
			build_out_dir: "dist"
			build_script:  "build"
		}
	}
}
