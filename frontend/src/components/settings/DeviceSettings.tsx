import { useState } from 'react';
import DeviceInfo from '@/components/baby/DeviceInfo';
import type { Baby } from '@/types/api';
import { displayName } from '@/lib/utils';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

interface DeviceSettingsProps {
  babies: Baby[];
}

export default function DeviceSettings({ babies }: DeviceSettingsProps) {
  const [activeTab, setActiveTab] = useState(0);

  if (babies.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        No devices found. Please ensure your Nanit account is authenticated and devices are connected.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Device Tabs */}
      {babies.length > 1 && (
        <Tabs value={String(activeTab)} onValueChange={(value) => setActiveTab(Number(value))}>
          <TabsList variant="line" className="overflow-x-auto">
            {babies.map((baby, index) => (
              <TabsTrigger key={baby.uid} value={String(index)}>
                {displayName(baby)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      )}

      {/* Device Content */}
      <div className="space-y-4">
        {babies.length === 1 ? (
          <div>
            <h4 className="text-lg font-medium mb-4">{babies[0].name}</h4>
            <DeviceInfo baby={babies[0]} />
          </div>
        ) : (
          <div>
            <h4 className="text-lg font-medium mb-4">{babies[activeTab]?.name}</h4>
            <DeviceInfo baby={babies[activeTab]} />
          </div>
        )}
      </div>

      {/* Device Summary for Multiple Devices */}
      {babies.length > 1 && (
        <Card size="sm" className="mt-8 bg-muted/50">
          <CardHeader>
            <CardTitle>Device Summary</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {babies.map((baby) => (
                <div key={baby.uid} className="bg-card rounded-lg p-3 border">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium">{displayName(baby)}</p>
                      <p className="text-xs text-muted-foreground">{baby.uid}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full ${
                        baby.websocket_alive ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-red dark:bg-ctp-red'
                      }`} />
                      <span className="text-xs text-muted-foreground">
                        {baby.websocket_alive ? 'Online' : 'Offline'}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
