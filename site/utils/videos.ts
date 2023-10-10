import { Redis } from 'ioredis';

const redisPromise = (async () => {
    if (process.env.REDIS_URL) {
        const redis = new Redis(
            process.env.REDIS_URL,
            {
                // keyPrefix: 'yt-data:',
                lazyConnect: true,
            }
        );
        redis.on('error', (err) => {
            console.error('redis error', err);
        });
        redis.on('close', () => {
            console.log('redis disconnected');
        })
        return redis.connect().then(() => {
            console.log('redis connected');
            return redis;
        }).catch(err => {
            console.error('redis connection error', err);
            return null;
        });
    }
    return null;
})();

export const revalidate = 60 * 60; // seconds, invalidated every hour

export type YTVideo = {
    id: string,
    title: string,
    thumbnail: {
        url: string,
        width: number,
        height: number,
    },
    dislikes: number,
    likes: number,
    views: number,
    published_date: string,
};

export function createYTVideo(data: any): YTVideo {
    return {
        id: typeof data.id === 'string' ? data.id : '',
        title: typeof data.title === 'string' ? data.title : '',
        dislikes: typeof data.dislikes === 'number' ? data.dislikes : 0,
        likes: typeof data.likes === 'number' ? data.likes : 0,
        views: typeof data.views === 'number' ? data.views : 0,
        published_date: typeof data.published_date === 'string' ? data.published_date : '',
        thumbnail: {
            url: typeof data.thumbnail?.url === 'string' ?  data.thumbnail.url : '',
            width: typeof data.thumbnail?.width === 'number' ?  data.thumbnail.width : 0,
            height: typeof data.thumbnail?.height === 'number' ?  data.thumbnail.height : 0,
        }
    };
}

export async function getYTChannelKeys() {
    const redis = await redisPromise;
    if (!redis) return null;

    return redis.keys('*')
        .then((keys) => keys.length ? keys : null)
        .catch(err => {
            console.error('channels fetch failed', err);
            return null;
        })
}

export async function getYTVideoFields(key: string) {
    const redis = await redisPromise;
    if (!redis || !key) return null;

    return redis.hkeys(key)
        .then(fields => fields.length ? fields : null)
        .catch(err => {
            console.error('videos fetch failed', err);
            return null;
        });
};

export async function getYTVideo(key: string, field: string) {
    const redis = await redisPromise;
    if (!redis) return null;

    return redis.hget(key, field)
        .then(dataStr => {
            if (!dataStr) return null;
            const data = JSON.parse(dataStr);
            return createYTVideo(data);
        })
        .catch(err => {
            console.error(`video [${key}] fetch failed`, err);
            return null;
        });
}