/* English and Hindi. Keys are the English strings, so untranslated text falls back to English.
   Translated: navigation, the citizen SOS screen and public alerts. The control-room screens stay in English. */
import { useStore } from "./state/store.js";

const HI = {
  // navigation
  "My assignment": "मेरा कार्य", Overview: "अवलोकन", Incidents: "घटनाएँ", Map: "मानचित्र", Routes: "मार्ग", Hospitals: "अस्पताल", Pharmacies: "दवा दुकानें",
  "Citizen SOS": "नागरिक SOS", Shelters: "आश्रय स्थल", Responders: "बचाव दल", "Early warning": "पूर्व चेतावनी", Infrastructure: "अवसंरचना", Evaluation: "मूल्यांकन",
  Modules: "मॉड्यूल", Architecture: "संरचना", "Situation report": "स्थिति रिपोर्ट", "Audit trail": "ऑडिट लॉग", Sources: "स्रोत", Settings: "सेटिंग्स", Collapse: "छोटा करें",
  "Dehradun control room": "देहरादून नियंत्रण कक्ष", "Towers up": "टावर चालू", "Towers down": "टावर बंद", "Device offline": "डिवाइस ऑफ़लाइन",
  // citizen SOS
  "Network available": "नेटवर्क उपलब्ध", "No network": "नेटवर्क नहीं है", held: "रुका हुआ",
  "Hold for one second, so a pocket cannot send it. We ask three short questions after.": "एक सेकंड दबाए रखें, ताकि जेब में गलती से न दब जाए। इसके बाद हम तीन छोटे सवाल पूछेंगे।",
  "Device location (test)": "डिवाइस का स्थान (परीक्षण)", "What is happening?": "क्या हो रहा है?", "How many people?": "कितने लोग हैं?", "Is anyone injured?": "क्या कोई घायल है?",
  Flood: "बाढ़", Landslide: "भूस्खलन", Trapped: "फँसे हैं", Medical: "चिकित्सा", No: "नहीं", Yes: "हाँ",
  "Send request": "अनुरोध भेजें", Back: "वापस", "Held on this device": "इस फ़ोन में सुरक्षित है", "Help is assigned": "मदद भेज दी गई है",
  "No network. The request will pass to nearby phones until one reaches a tower.": "नेटवर्क नहीं है। अनुरोध पास के फ़ोनों से होते हुए टावर तक पहुँचेगा।",
  "is coming. You are going to": "आ रही है। आपको यहाँ ले जाया जाएगा:", "The next free unit": "अगली उपलब्ध गाड़ी", "Raise another": "नया अनुरोध",
  "Install this app": "ऐप इंस्टॉल करें", "Hold for one second to send SOS": "SOS भेजने के लिए एक सेकंड दबाए रखें"
};
export const ALERT_TEXT = {
  en: "River rising. Move away from the bank to higher ground now.",
  hi: "नदी का जलस्तर बढ़ रहा है। किनारे से हटकर तुरंत ऊँचे स्थान पर जाएँ।"
};
export const tr = (lang, s) => (lang === "hi" && HI[s]) || s;
export function useT() { const lang = useStore((s) => s.lang); return (s) => tr(lang, s); }
