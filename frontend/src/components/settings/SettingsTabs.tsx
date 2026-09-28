import { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export interface SettingsTab {
  id: string;
  label: string;
  icon: LucideIcon;
  content: ReactNode;
  disabled?: boolean;
}

interface SettingsTabsProps {
  tabs: SettingsTab[];
  defaultTab?: string;
}

export default function SettingsTabs({ tabs, defaultTab }: SettingsTabsProps) {
  return (
    <Tabs defaultValue={defaultTab || tabs[0]?.id} className="gap-6">
      <TabsList variant="line" className="overflow-x-auto">
        {tabs.map((tab) => (
          <TabsTrigger key={tab.id} value={tab.id} disabled={tab.disabled}>
            <tab.icon />
            {tab.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {tabs.map((tab) => (
        <TabsContent key={tab.id} value={tab.id}>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <tab.icon className="size-5" />
                {tab.label}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">{tab.content}</CardContent>
          </Card>
        </TabsContent>
      ))}
    </Tabs>
  );
}
