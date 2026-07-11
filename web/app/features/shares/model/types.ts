export interface ShareItem {
  id: string;
  title: string;
  targetKind?: "file" | "subscription";
  targetName: string;
  targetFormat?: string;
  validFrom?: string;
  validUntil?: string;
  ageRecipient?: string;
  maxUses?: number;
  useCount?: number;
  status: "valid" | "upcoming" | "expired" | "exhausted";
  publicUrl: string;
}

export interface ShareTarget {
  kind: "file" | "subscription";
  name: string;
}
