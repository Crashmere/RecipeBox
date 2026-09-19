export interface Ingredient {name:string;amount:string}
export interface CookLog {id:string;date:string;note:string;rating:number}
export interface Entry {id:string;kind:'recipe'|'pantry';revision:number;name:string;category:string;notes:string;tags:string[];ingredients:Ingredient[];steps:string[];minutes:number;servings:number;favorite:boolean;logs:CookLog[];photoIds:string[];quantity:number;unit:string;location:string;purchaseDate:string;expiryDate:string;createdAt:string;updatedAt:string;deletedAt:string|null}
export const blank=(kind:Entry['kind']='recipe'):Entry=>({id:'',kind,revision:0,name:'',category:kind==='recipe'?'家常菜':'方便速食',notes:'',tags:[],ingredients:[],steps:[],minutes:0,servings:0,favorite:false,logs:[],photoIds:[],quantity:1,unit:'份',location:'',purchaseDate:'',expiryDate:'',createdAt:'',updatedAt:'',deletedAt:null});
export const today=()=>{const d=new Date();return [d.getFullYear(),String(d.getMonth()+1).padStart(2,'0'),String(d.getDate()).padStart(2,'0')].join('-')};
export const expiryDays=(date:string)=>date?Math.round((Date.parse(date+'T00:00:00Z')-Date.parse(today()+'T00:00:00Z'))/86400000):null;
export const expiryText=(e:Entry)=>{if(!e.quantity)return '已吃完';const d=expiryDays(e.expiryDate);return d===null?'未设置到期日':d<0?'已过期 '+(-d)+' 天':d===0?'今天到期':d<=30?d+' 天后到期':e.expiryDate+' 到期'};
export const recipeCategories=['家常菜','荤菜','素菜','汤羹','主食','早餐','烘焙甜品','凉菜','其他'];
export const pantryCategories=['方便速食','米面粮油','罐头干货','零食饮品','调味品','其他'];
