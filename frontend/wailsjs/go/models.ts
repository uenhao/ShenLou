export namespace main {
	
	export class FavStation {
	    name: string;
	    url: string;
	    playlists: string[];
	
	    static createFrom(source: any = {}) {
	        return new FavStation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.playlists = source["playlists"];
	    }
	}
	export class HistoryEntry {
	    name: string;
	    url: string;
	    // Go type: time
	    at: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.at = this.convertValues(source["at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlaylistInfo {
	    name: string;
	    file: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new PlaylistInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.file = source["file"];
	        this.count = source["count"];
	    }
	}
	export class RecordingInfo {
	    id: string;
	    name: string;
	    url: string;
	    file: string;
	    size: number;
	    startedAt: string;
	    duration: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RecordingInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.file = source["file"];
	        this.size = source["size"];
	        this.startedAt = source["startedAt"];
	        this.duration = source["duration"];
	        this.active = source["active"];
	    }
	}
	export class Station {
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Station(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class TvChannel {
	    name: string;
	    logo: string;
	    country: string;
	    categories: string[];
	    urls: string[];
	
	    static createFrom(source: any = {}) {
	        return new TvChannel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.logo = source["logo"];
	        this.country = source["country"];
	        this.categories = source["categories"];
	        this.urls = source["urls"];
	    }
	}

}

