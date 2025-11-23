import type {
  DurationFormat as DurationFormatInterface,
  DurationFormatConstructor as DurationFormatConstructorType,
  DurationFormatOptions as DurationFormatOptionsType,
  DurationFormatPart as DurationFormatPartType,
  DurationInput as DurationInputType,
  ResolvedDurationFormatOptions as ResolvedDurationFormatOptionsType,
} from "@formatjs/intl-durationformat/src/types";

declare global {
  namespace Intl {
    type DurationFormatOptions = DurationFormatOptionsType;
    type ResolvedDurationFormatOptions = ResolvedDurationFormatOptionsType;
    type DurationFormatPart = DurationFormatPartType;
    type DurationInput = DurationInputType;
    type DurationFormat = DurationFormatInterface;
    const DurationFormat: DurationFormatConstructorType;
  }
}

export {};
