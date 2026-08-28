import type { components } from "@/lib/api/schema";

/**
 * Shared fixtures, typed against the generated contract. The types stop a
 * fixture inventing a field; `contractJson` stops it from being the wrong
 * shape at runtime. Vietnamese names throughout, since that is the product.
 */

export const studentUser: components["schemas"]["User"] = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
  email: "hocvien@example.com",
  fullName: "Nguyễn Văn An",
  role: "student",
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
};

export const adminUser: components["schemas"]["User"] = {
  ...studentUser,
  id: "019535d9-3df7-79fb-b466-fa907fa17f9f",
  email: "thuong@example.com",
  fullName: "Thuong",
  role: "admin",
};

export const sampleClass: components["schemas"]["Class"] = {
  id: "019535da-0000-7000-8000-000000000001",
  name: "Tiếng Anh giao tiếp - Lớp A",
  description: null,
  studentCount: 12,
  selfJoinEnabled: true,
  joinCode: null,
  createdAt: "2026-01-01T00:00:00Z",
};
