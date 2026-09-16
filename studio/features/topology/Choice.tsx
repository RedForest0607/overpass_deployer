'use client';
import {
  Select,
  SelectTrigger,
  SelectContent,
  SelectItem,
} from '@/components/ui/select';

type ChoiceOption =
  | string
  | {
      label: string;
      value: string;
      description?: string;
    };

export default function Choice({
  value,
  options,
  onChange,
  label,
}: {
  value: string;
  options: ChoiceOption[];
  onChange: (value: string) => void;
  label: string;
}) {
  const selected = options.find(
    (option) => (typeof option === 'string' ? option : option.value) === value,
  );

  return (
    <Select
      value={value}
      onValueChange={(v) => {
        if (v) onChange(v);
      }}
    >
      <SelectTrigger aria-label={label} className="choice">
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          {renderOption(selected ?? value)}
        </span>
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem
            key={typeof o === 'string' ? o : o.value}
            value={typeof o === 'string' ? o : o.value}
          >
            {renderOption(o)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function renderOption(option: ChoiceOption) {
  if (typeof option === 'string') return option;
  return (
    <>
      <span className="min-w-0 truncate font-medium">{option.label}</span>
      {option.description && (
        <span className="ml-auto shrink-0 text-[10px] font-normal text-muted-foreground">
          {option.description}
        </span>
      )}
    </>
  );
}
