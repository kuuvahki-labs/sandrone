import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

interface ProbeURLPreset {
  provider: string;
  url: string;
}

const probeURLPresets: ProbeURLPreset[] = [
  { provider: "Google", url: "http://www.gstatic.com/generate_204" },
  { provider: "Apple", url: "http://captive.apple.com/hotspot-detect.html" },
  { provider: "Cloudflare", url: "http://cp.cloudflare.com/generate_204" },
  { provider: "Microsoft", url: "http://www.msftconnecttest.com/connecttest.txt" },
  { provider: "华为", url: "http://connectivitycheck.platform.hicloud.com/generate_204" },
];

export interface ProbeURLFieldProps {
  className?: string;
  label: string;
  placeholder?: string;
  value: string;
  onChange: (value: string) => void;
}

export function ProbeURLField({ className, label, onChange, placeholder, value }: ProbeURLFieldProps) {
  const selected = probeURLPresets.find((option) => option.url === value) ?? value;
  return (
    <Autocomplete<ProbeURLPreset, false, false, true>
      autoHighlight
      autoSelect
      className={className}
      freeSolo
      inputValue={value}
      options={probeURLPresets}
      value={selected}
      getOptionLabel={(option) => typeof option === "string" ? option : option.url}
      isOptionEqualToValue={(option, current) =>
        typeof current !== "string" && option.url === current.url
      }
      renderInput={(params) => <TextField {...params} fullWidth label={label} placeholder={placeholder} />}
      renderOption={(props, option) => (
        <li {...props} aria-label={`${option.provider} ${option.url}`} key={option.provider}>
          <span className="grid min-w-0 gap-0.5">
            <Typography component="span" variant="body2">{option.provider}</Typography>
            <Typography className="break-words" color="text.secondary" component="span" variant="caption">
              {option.url}
            </Typography>
          </span>
        </li>
      )}
      onChange={(_event, next) =>
        onChange(typeof next === "string" ? next : next?.url ?? "")
      }
      onInputChange={(_event, next) => onChange(next)}
    />
  );
}
