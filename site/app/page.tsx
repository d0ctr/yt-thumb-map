import { Suspense } from 'react';
import { getYTChannelKeys, getYTVideoFields } from '../utils/videos';
import Thumbnail from './thumbnail';
import { channel } from 'diagnostics_channel';

export default async function Page() {
    const channelKeys = await getYTChannelKeys();
    const videos: [string, string[]][] = [['', []]];
    channelKeys && await Promise.all(channelKeys.map(async channelKey => {
        const fields = await getYTVideoFields(channelKey);
        if (!fields || !fields.length) return;
        videos.push([channelKey, fields]);
    }));
    return (
        <section>
        {
            videos?.length ?
            <Suspense fallback={<p>Loading thumbnails...</p>}>
                {videos.map(([channelKey, fields]) => 
                    fields.map(field => 
                        <Thumbnail key={`${channelKey}:${field}`} channelKey={channelKey} videoField={field} />))}
            </Suspense> :
            <p>No videos</p>
        }
        </section>
    )
}
