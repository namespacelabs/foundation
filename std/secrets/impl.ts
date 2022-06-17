import {Secret, Value, SecretDevMap} from "@namespacelabs.dev-foundation/std-secrets/provider_pb";
import yargs from "yargs/yargs";
import {promises as fs} from "fs";
import {isAbsolute, join} from "path";

const args = yargs(process.argv)
	.options({
		server_secrets_basepath: { type: "string" },
	})
	.parse();

export async function  provideSecret(req: Secret): Promise<Value> {
  const caller = "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple";

  const devMap = await loadBinaryDevMap(args.server_secrets_basepath!);

  console.log(`devMap: ${JSON.stringify(devMap.toObject())}`);

  const config = devMap.getConfigureList().find(c => c.getPackageName() === caller);
  if (!config) {
    if (req.getOptional()) {
      return new Value();
    } else {
      throw new Error(`No config for ${caller}`);
    }
  }

  const secret = config.getSecretList().filter(s => s.getName() === req.getName()).shift();

  if (secret) {
    if (!secret.getFromPath()) {
      throw new Error(`Secret ${secret.getName()} is not from path`);
    }
    if (!isAbsolute(secret.getFromPath())) {
      throw new Error(`Secret ${secret.getName()} is not absolute path`);
    }

    const value = new Value();
    value.setName(secret.getName());
    value.setPath(secret.getFromPath());
    return value;
  }

  if (req.getOptional()) {
    return new Value();
  }

  throw new Error(`No secret for ${caller}/${req.getName()}`);
}

async function loadBinaryDevMap(path: string): Promise<SecretDevMap> {
  const mapContents = await fs.readFile(join(path, "map.binarypb"));
  return SecretDevMap.deserializeBinary(mapContents);
}

