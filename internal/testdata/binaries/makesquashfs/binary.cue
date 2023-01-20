binary: {
	name: "makesquashfs"

	from: make_squashfs: {
		from: image_id: "busybox"
		target: "busybox.squashfs"
	}
}
