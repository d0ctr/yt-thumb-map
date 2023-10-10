import { getYTVideo } from '@/utils/videos';
import Image from 'next/image';

export default async function Thumbnail({ 
    channelKey,
    videoField
 }: {
    channelKey: string,
    videoField: string,
 }) {
    const video = await getYTVideo(channelKey, videoField);
    return video ?
        <div className='video'>
            <Image className='object-cover w-64 h-36' src={video.thumbnail.url} width={video.thumbnail.width} height={video.thumbnail.height} alt={video.title}/>
        </div> :
        <p>{`No video at ${channelKey}:${videoField}`}</p>;
}