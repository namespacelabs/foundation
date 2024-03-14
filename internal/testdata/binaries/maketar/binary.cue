binary: {
	name: "maketar"

	build_plan: [
		{
			make_fs_image: {
				from: image_id: "busybox"
				target: "busybox.tgz"
				kind:   "tgz"
			}
		},
		{
			make_fs_image: {
				from: image_id: "busybox"
				target: "busybox.tar"
				kind:   "tar"
			}
		},
		{
			make_fs_image: {
				from: image_id: "busybox"
				target: "busybox.ext4"
				kind:   "ext4"
				size: "1g"
			}
		},
		{
			make_fs_image: {
				from: image_id: "busybox"
				target: "busybox.ext4"
				kind:   "ext4"
				size: "1g"
				raw: true
			}
		},
	]
}
