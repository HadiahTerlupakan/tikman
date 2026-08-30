export interface Site {
  id: string;
  name: string;
  location: string;
  description: string;
  latitude?: number;
  longitude?: number;
  oltCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSiteDto {
  name: string;
  location?: string;
  description?: string;
  latitude?: number;
  longitude?: number;
}

export interface UpdateSiteDto {
  name?: string;
  location?: string;
  description?: string;
  latitude?: number;
  longitude?: number;
}
