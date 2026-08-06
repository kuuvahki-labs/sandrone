export interface ShareItem {
  id: string;
  title: string;
  targetKind?: "file" | "subscription";
  targetName: string;
  targetFormat?: string;
  validFrom?: string;
  validUntil?: string;
  ageRecipient?: string;
  status: "valid" | "upcoming" | "expired";
  publicUrl: string;
  formatFilenames?: Record<string, string>;
}

export interface ShareTarget {
  kind: "file" | "subscription";
  name: string;
}
