export enum DatabaseErrorType {
  Read = "read",
  Insert = "insert",
  Update = "update",
  Delete = "delete",
}

export enum DatabaseRepoName {
  MessageRepository = "MessageRepository",
}

export enum Direction {
  L1_TO_L2 = "L1_TO_L2",
  L2_TO_L1 = "L2_TO_L1",
}

export enum MessageStatus {
  SENT = "SENT",
  ANCHORED = "ANCHORED",
  PENDING = "PENDING",
  CLAIMED_SUCCESS = "CLAIMED_SUCCESS",
  CLAIMED_REVERTED = "CLAIMED_REVERTED",
  NON_EXECUTABLE = "NON_EXECUTABLE",
  ZERO_FEE = "ZERO_FEE",
  FEE_UNDERPRICED = "FEE_UNDERPRICED",
  EXCLUDED = "EXCLUDED",
}
