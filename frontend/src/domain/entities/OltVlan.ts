export interface OltVlanPort {
  slot: number;
  port: number;
  tagged: boolean;
}

export interface OltVlan {
  vlanId: number;
  name: string;
  ports: OltVlanPort[];
}
