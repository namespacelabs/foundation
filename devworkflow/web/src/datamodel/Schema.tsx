// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

export enum NodeKindEnum {
	SERVICE = 1,
	EXTENSION = 2,
}

export type NodeType = {
	package_name: string;
	kind: NodeKindEnum;
	export_service?: ExportedServiceType[];
	ingress: string;
	ingress_service_name: string;
};

export type ServerType = {
	package_name: string;
	name: string;
	id: string;
	cluster_admin: boolean;
	import?: string[];
	user_imports?: string[];
};

export enum EnvironmentPurposeEnum {
	DEVELOPMENT = 1,
	TESTING = 2,
	PRODUCTION = 3,
}

export type EnvironmentType = {
	name: string;
	runtime: string;
	purpose: EnvironmentPurposeEnum;
};

export enum EndpointTypeEnum {
	PRIVATE = 1,
	INTERNET_FACING = 2,
}

type EndpointType = {
	endpoint_owner: string;
	server_owner: string;
	port?: PortType;
	service_name: string;
	allocated_name: string;
	type: EndpointTypeEnum;
	service_metadata?: ServiceMetadataType[];
};

type ServiceMetadataType = {
	kind?: string;
	protocol: string;
};

export type ExportedServiceType = {
	proto: string[];
	proto_typename: string;
};

export type StackEntryStateType = {
	package_name: string;
	last_error?: string;
};

type EndpointSchemaType = {
	port?: PortType;
	service_name: string;
	server_owner?: string;
};

type PortType = {
	name: string;
	container_port: number;
};

export type ForwardedPort = {
	local_port: number;
	container_port: number;
	endpoint: EndpointSchemaType;
};

export type DataType = {
	abs_root: string;
	workspace: WorkspaceType;
	env: EnvironmentType;
	available_env: EnvironmentType[];
	stack?: StackType;
	current: StackEntryType;
	state?: StackEntryStateType[];
	forwarded_port?: ForwardedPort[];
};

export type StackType = {
	entry?: StackEntryType[];
	endpoint?: EndpointType[];
};

export type StackEntryType = {
	server: ServerType;
	node?: NodeType[];
};

export type WorkspaceType = {
	module_name: string;
};
