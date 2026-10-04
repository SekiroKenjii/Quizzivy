import { describe, expect, it } from "vitest";
import en from "@/lib/i18n/locales/en.json";
import vi from "@/lib/i18n/locales/vi.json";
import { MAX_AUDIO_BYTES, MAX_IMAGE_BYTES } from "@/features/media/limits";

const megabytes = (bytes: number) => `${bytes / (1024 * 1024)} MB`;

describe("the sentences that name an upload's size name the limit in force", () => {
  for (const [language, locale] of Object.entries({ vi, en })) {
    it(`${language}: audio is ${megabytes(MAX_AUDIO_BYTES)}`, () => {
      for (const sentence of [
        locale.media.dropHint,
        locale.media.rejectSize,
        locale.questionEditor.mediaHint,
      ]) {
        expect(sentence).toContain(megabytes(MAX_AUDIO_BYTES));
        expect(sentence).not.toContain(megabytes(MAX_IMAGE_BYTES));
      }
    });

    it(`${language}: an image is ${megabytes(MAX_IMAGE_BYTES)}`, () => {
      expect(locale.groups.imageLimit).toContain(megabytes(MAX_IMAGE_BYTES));
    });
  }
});
