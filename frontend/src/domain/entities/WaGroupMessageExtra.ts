/** A member who reacted or voted. */
export interface WaGroupMemberRef {
  jid: string;
  name: string;
  isSelf: boolean;
}

/** Everyone who reacted with one emoji. */
export interface WaGroupReaction {
  emoji: string;
  count: number;
  reactors: WaGroupMemberRef[];
}

export interface WaGroupLocation {
  latitude: number;
  longitude: number;
  name: string;
  address: string;
  /** "" when WhatsApp sent no link of its own. */
  url: string;
}

export interface WaGroupContactCard {
  displayName: string;
  phones: string[];
}

export interface WaGroupPollOption {
  name: string;
  voters: WaGroupMemberRef[];
}

export interface WaGroupPoll {
  question: string;
  /** 0 is any number of options, 1 a single choice. */
  selectableCount: number;
  options: WaGroupPollOption[];
}

/** The structured part of a place, contact or poll message. Which shape it
 * takes follows the message's kind; the fields are optional because the
 * server sends only the one that kind carries. */
export interface WaGroupMessageExtra
  extends Partial<WaGroupLocation>,
    Partial<WaGroupPoll> {
  contacts?: WaGroupContactCard[];
}

/** One member's copy of a message we sent, for "Info pesan". */
export interface WaGroupReceipt {
  jid: string;
  name: string;
  deliveredAt?: string;
  readAt?: string;
}

/** The body of POST /groups/:id/polls. */
export interface WaGroupPollRequest {
  question: string;
  options: string[];
  selectableCount: number;
}
