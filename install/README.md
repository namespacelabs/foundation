# Deploying the Linux installation script to AWS

Update the version variable in the shell script and deploy the script to the private s3 bucket
hosting installation scripts with a public ACL.

```bash
sed -i -r "s/^(VERSION=).*/\10.0.48/" install.sh
```

Configure the `prod-main` account and copy the modified installation script to the bucket:

```bash
aws configure --profile prod-main sso

aws s3 ls --profile prod-main

aws s3 cp install.sh s3://nsinstall/install.sh --acl public-read --profile prod-main
```

Verify everything works as expected by downloading the public installation script:

```bash
curl -fsSL https://nsinstall.s3.us-east-2.amazonaws.com/install.sh | sh
```
