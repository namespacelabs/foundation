source = ["./dist/macos_darwin_amd64/fn","./dist/macos_darwin_arm64/fn"]
bundle_id = "com.namespacelabs.foundation"

apple_id {
  username = "hugosantos@gmail.com"
  password = "@env:APPLE_PASSWORD"
}

sign {
  application_identity = "Developer ID Application: Namespace Labs Inc"
}

zip {
  output_path = "./dist/fn_macos.zip"
}

dmg {
  output_path = "./dist/fn_macos.dmg"
  volume_name = "Foundation"
}