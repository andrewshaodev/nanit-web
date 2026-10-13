import { useState } from 'react';
import { Check, Copy, Eye, EyeOff, Wand2 } from 'lucide-react';
import { copyToClipboard } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

// Letters and digits that can't be mistaken for each other (no 0/O, 1/l/I)
const ALPHABET = 'abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';

// A random password of 20 characters (about 117 bits)
export function generateSecret(length = 20): string {
  const out: string[] = [];
  const bytes = new Uint8Array(1);
  // Rejection sampling, so every character is equally likely
  const limit = 256 - (256 % ALPHABET.length);
  while (out.length < length) {
    crypto.getRandomValues(bytes);
    if (bytes[0] < limit) out.push(ALPHABET[bytes[0] % ALPHABET.length]);
  }
  return out.join('');
}

interface NewCredentialInputProps {
  id: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

// A new-password field with Generate, show/hide and copy
export default function NewCredentialInput({ id, value, onChange, placeholder }: NewCredentialInputProps) {
  const [visible, setVisible] = useState(false);
  const [copied, setCopied] = useState(false);

  const generate = () => {
    onChange(generateSecret());
    // A generated password has to be seen to be saved somewhere
    setVisible(true);
  };

  const copy = async () => {
    if (value && (await copyToClipboard(value))) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <Input
          type={visible ? 'text' : 'password'}
          id={id}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
          placeholder={placeholder}
          className={visible ? 'font-mono' : undefined}
        />
        <Button
          type="button"
          size="icon"
          variant="outline"
          onClick={() => setVisible(!visible)}
          title={visible ? 'Hide password' : 'Show password'}
          aria-label={visible ? 'Hide password' : 'Show password'}
        >
          {visible ? <EyeOff /> : <Eye />}
        </Button>
        <Button
          type="button"
          size="icon"
          variant={copied ? 'success' : 'outline'}
          onClick={copy}
          disabled={!value}
          title="Copy password"
          aria-label="Copy password"
        >
          {copied ? <Check /> : <Copy />}
        </Button>
      </div>
      <Button type="button" size="sm" variant="outline" onClick={generate}>
        <Wand2 />
        Generate a strong password
      </Button>
    </div>
  );
}
