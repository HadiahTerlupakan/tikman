export interface Site {
  id: string;
  name: string;
  location: string;
  description: string;
  oltCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSiteDto {
  name: string;
  location?: string;
  description?: string;
}

export interface UpdateSiteDto {
  name?: string;
  location?: string;
  description?: string;
}
