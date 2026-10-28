const CATEGORIES = {
  PMD: { label: 'PMD', description: 'Plastic, metal, drink cartons', color: 'bg-blue-100 text-blue-800' },
  ORGANIC: { label: 'Organic', description: 'Food waste, plants', color: 'bg-green-100 text-green-800' },
  PAPER: { label: 'Paper', description: 'Paper, cardboard', color: 'bg-amber-100 text-amber-800' },
  GENERAL_WASTE: { label: 'General waste', description: 'Non-recyclable waste', color: 'bg-gray-200 text-gray-800' },
  UNSUPPORTED: { label: 'Not supported', description: 'This bin cannot sort it', color: 'bg-red-100 text-red-800' },
}

export function categoryInfo(category) {
  return CATEGORIES[category] ?? { label: category || 'Unknown', description: '', color: 'bg-gray-100 text-gray-700' }
}
